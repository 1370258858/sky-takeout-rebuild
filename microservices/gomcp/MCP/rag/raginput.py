from __future__ import annotations

import argparse
import hashlib
import re
from dataclasses import dataclass
from pathlib import Path
from typing import TypedDict

import chromadb
from docx import Document
from langgraph.graph import END, StateGraph


BASE_DIR = Path(__file__).resolve().parent
DATA_FILE = BASE_DIR / "data" / "origin_data.docx"
ZHENGXING_MD_FILE = BASE_DIR / "data" / "zhengxing.md"
COLLECTION_NAME = "slang_sku_kb"


@dataclass
class SlangEntry:
	slang: str
	sku: str
	product_name: str
	category: str


@dataclass
class SemanticMapping:
	utterance: str
	intent: str
	slots: str
	candidate_category: str
	candidate_skus: list[str]
	primary_sku: str


@dataclass
class HardMapping:
	slang: str
	sku: str
	good_id: int
	product_name: str


def _good_id_from_sku(sku: str) -> int:
	parts = sku.rsplit("_", 1)
	if len(parts) != 2:
		return 0
	if not parts[1].isdigit():
		return 0
	return int(parts[1])


def read_source_doc(input_file: Path = DATA_FILE) -> list[SlangEntry]:
	if not input_file.exists():
		raise FileNotFoundError(f"source file not found: {input_file}")

	doc = Document(str(input_file))
	rows: list[SlangEntry] = []
	for p in doc.paragraphs:
		raw = p.text.strip()
		if not raw or raw.startswith("黑话-SKU") or raw.startswith("格式："):
			continue
		parts = [x.strip() for x in raw.split("|")]
		if len(parts) != 4:
			continue
		rows.append(
			SlangEntry(
				slang=parts[0],
				sku=parts[1],
				product_name=parts[2],
				category=parts[3],
			)
		)
	return rows


def _semantic_doc_id(utterance: str, primary_sku: str) -> str:
	digest = hashlib.sha1(f"{utterance}|{primary_sku}".encode("utf-8")).hexdigest()[:12]
	return f"semantic::{digest}"


def _hard_doc_id(slang: str, sku: str) -> str:
	digest = hashlib.sha1(f"{slang}|{sku}".encode("utf-8")).hexdigest()[:12]
	return f"hard::{digest}"


def _split_md_row(line: str) -> list[str]:
	text = line.strip()
	if not text or "|" not in text:
		return []
	if text.startswith("|"):
		text = text[1:]
	if text.endswith("|"):
		text = text[:-1]
	return [cell.strip() for cell in text.split("|")]


def _extract_section_lines(md_text: str, section_title: str) -> list[str]:
	pattern = re.compile(rf"^##\s+{re.escape(section_title)}\s*$", re.MULTILINE)
	match = pattern.search(md_text)
	if not match:
		return []
	start = match.end()
	rest = md_text[start:]
	next_header = re.search(r"^##\s+", rest, re.MULTILINE)
	section = rest[: next_header.start()] if next_header else rest
	return [line for line in section.splitlines() if line.strip()]


def _extract_section_lines_by_title_prefix(md_text: str, title_prefix: str) -> list[str]:
	pattern = re.compile(rf"^##\s+{re.escape(title_prefix)}.*$", re.MULTILINE)
	match = pattern.search(md_text)
	if not match:
		return []
	start = match.end()
	rest = md_text[start:]
	next_header = re.search(r"^##\s+", rest, re.MULTILINE)
	section = rest[: next_header.start()] if next_header else rest
	return [line for line in section.splitlines() if line.strip()]


def parse_semantic_mappings_from_md(md_file: Path = ZHENGXING_MD_FILE) -> list[SemanticMapping]:
	if not md_file.exists():
		raise FileNotFoundError(f"markdown file not found: {md_file}")

	md_text = md_file.read_text(encoding="utf-8")
	lines = _extract_section_lines(md_text, "语义模糊映射（意图+槽位+候选类目）")
	if not lines:
		raise ValueError("semantic mapping section not found in markdown")

	mappings: list[SemanticMapping] = []
	for line in lines:
		cells = _split_md_row(line)
		if len(cells) < 6:
			continue
		if cells[0] == "用户口语" or set(cells[0]) == {"-"}:
			continue

		utterance = cells[0].strip()
		if not utterance:
			continue

		candidate_skus = [
			s.strip()
			for s in re.split(r"[,，]", cells[4])
			if s.strip().startswith("SKU_")
		]
		primary_sku = cells[5].strip()
		if not primary_sku.startswith("SKU_"):
			continue

		mappings.append(
			SemanticMapping(
				utterance=utterance,
				intent=cells[1].strip(),
				slots=cells[2].strip(),
				candidate_category=cells[3].strip(),
				candidate_skus=candidate_skus,
				primary_sku=primary_sku,
			)
		)

	if not mappings:
		raise ValueError("no semantic mappings parsed from markdown")
	return mappings


def parse_hard_mappings_from_md(md_file: Path = ZHENGXING_MD_FILE) -> list[HardMapping]:
	if not md_file.exists():
		raise FileNotFoundError(f"markdown file not found: {md_file}")

	md_text = md_file.read_text(encoding="utf-8")
	lines = _extract_section_lines_by_title_prefix(md_text, "黑话 -> SKU 映射")
	if not lines:
		raise ValueError("hard mapping section not found in markdown")

	mappings: list[HardMapping] = []
	for line in lines:
		cells = _split_md_row(line)
		if len(cells) < 5:
			continue
		if cells[0] in {"序号", "----", "---"} or set(cells[0]) == {"-"}:
			continue

		slang = cells[1].strip()
		sku = cells[2].strip()
		good_id_text = cells[3].strip()
		product_name = cells[4].strip()

		if not slang or not sku.startswith("SKU_"):
			continue
		if not good_id_text.isdigit():
			continue

		mappings.append(
			HardMapping(
				slang=slang,
				sku=sku,
				good_id=int(good_id_text),
				product_name=product_name,
			)
		)

	if not mappings:
		raise ValueError("no hard mappings parsed from markdown")
	return mappings


def build_semantic_chunks(
	mappings: list[SemanticMapping],
	hard_mappings: list[HardMapping],
	source_file: str,
) -> tuple[list[str], list[str], list[dict]]:
	ids: list[str] = []
	docs: list[str] = []
	metas: list[dict] = []
	hard_name_by_sku = {m.sku: m.product_name for m in hard_mappings}

	for m in mappings:
		product_name = hard_name_by_sku.get(m.primary_sku, "UNKNOWN")
		category = m.candidate_category or "未分类"

		ids.append(_semantic_doc_id(m.utterance, m.primary_sku))
		docs.append(
			"\n".join(
				[
					f"用户口语: {m.utterance}",
					f"意图: {m.intent}",
					f"槽位: {m.slots}",
					f"候选类目: {m.candidate_category}",
					"候选SKU: " + "、".join(m.candidate_skus),
					f"主推荐SKU: {m.primary_sku}",
					f"主推荐品名: {product_name}",
				]
			)
		)
		metas.append(
			{
				"doc_type": "semantic_entry",
				"source_file": source_file,
				"utterance": m.utterance,
				"intent": m.intent,
				"slots": m.slots,
				"candidate_category": m.candidate_category,
				"primary_sku": m.primary_sku,
				"sku": m.primary_sku,
				"product_name": product_name,
				"category": category,
			}
		)

	return ids, docs, metas


def build_hard_mapping_chunks(
	mappings: list[HardMapping],
	source_file: str,
) -> tuple[list[str], list[str], list[dict]]:
	ids: list[str] = []
	docs: list[str] = []
	metas: list[dict] = []

	for m in mappings:
		category = "未分类"
		product_name = m.product_name

		ids.append(_hard_doc_id(m.slang, m.sku))
		docs.append(
			"\n".join(
				[
					f"黑话: {m.slang}",
					f"标准SKU: {m.sku}",
					f"good_id: {m.good_id}",
					f"标准品名: {product_name}",
					f"品类: {category}",
					"用途: 高精度黑话映射",
				]
			)
		)
		metas.append(
			{
				"doc_type": "hard_mapping",
				"source_file": source_file,
				"slang": m.slang,
				"sku": m.sku,
				"good_id": m.good_id,
				"product_name": product_name,
				"category": category,
			}
		)

	return ids, docs, metas


def _upsert_source_chunks(
	collection,
	ids: list[str],
	docs: list[str],
	metas: list[dict],
	source_file: str,
) -> tuple[int, int]:
	existing = collection.get(where={"source_file": source_file}, include=["metadatas"])
	existing_ids = set(existing.get("ids", []))
	new_ids = set(ids)
	stale_ids = sorted(existing_ids - new_ids)
	if stale_ids:
		collection.delete(ids=stale_ids)
	collection.upsert(ids=ids, documents=docs, metadatas=metas)
	return len(stale_ids), len(ids)


class IngestState(TypedDict, total=False):
	host: str
	port: int
	dry_run: bool
	input_file: str
	input_type: str
	md_file: str
	doc_file: str
	hard_mappings: list[HardMapping]
	semantic_mappings: list[SemanticMapping]
	hard_chunks: tuple[list[str], list[str], list[dict]]
	semantic_chunks: tuple[list[str], list[str], list[dict]]
	result: dict


def _route_input_type(state: IngestState) -> str:
	input_file = state.get("input_file") or state.get("md_file") or state.get("doc_file")
	if not input_file:
		return "md"
	suffix = Path(input_file).suffix.lower()
	if suffix in {".docx", ".doc"}:
		return "doc"
	return "md"


def _node_use_md_input(state: IngestState) -> IngestState:
	input_file = state.get("input_file") or state.get("md_file")
	if not input_file:
		input_file = str(ZHENGXING_MD_FILE)
	return {"md_file": str(Path(input_file)), "input_type": "md"}


def _convert_doc_to_md(
	doc_file: Path,
	md_file: Path,
) -> None:
	rows = read_source_doc(doc_file)
	md_file.parent.mkdir(parents=True, exist_ok=True)
	lines: list[str] = []
	lines.append("# 自动转换黑话与SKU映射")
	lines.append("")
	lines.append(f"- 来源文档: {doc_file.name}")
	lines.append("")
	lines.append(f"## 黑话 -> SKU 映射（{len(rows)}条）")
	lines.append("")
	lines.append("| 序号 | 黑话/说法 | SKU | good_id | 标准商品名 |")
	lines.append("|---|---|---|---|---|")
	for idx, item in enumerate(rows, start=1):
		good_id = _good_id_from_sku(item.sku)
		lines.append(
			f"| {idx} | {item.slang} | {item.sku} | {good_id} | {item.product_name} |"
		)
	lines.append("")
	lines.append(f"## 语义模糊映射（意图+槽位+候选类目）")
	lines.append("")
	lines.append("| 用户口语 | 意图 | 槽位抽取 | 候选类目 | 候选SKU（Top3） | 主推荐SKU |")
	lines.append("|---|---|---|---|---|---|")
	for item in rows:
		category = item.category
		lines.append(
			f"| {item.slang} | 点单-黑话映射 | source=doc_auto | {category} | {item.sku} | {item.sku} |"
		)
	md_file.write_text("\n".join(lines) + "\n", encoding="utf-8")


def _node_convert_doc_to_md(state: IngestState) -> IngestState:
	input_file = state.get("input_file") or state.get("doc_file")
	if not input_file:
		raise ValueError("doc input required for doc routing")
	doc_file = Path(input_file)
	converted_file = doc_file.with_suffix(".converted.md")
	_convert_doc_to_md(doc_file, converted_file)
	return {
		"doc_file": str(doc_file),
		"md_file": str(converted_file),
		"input_type": "doc",
	}


def _node_parse_md_hard(state: IngestState) -> IngestState:
	md_file = Path(state["md_file"])
	hard_mappings = parse_hard_mappings_from_md(md_file)
	return {"hard_mappings": hard_mappings}


def _node_parse_md_semantic(state: IngestState) -> IngestState:
	md_file = Path(state["md_file"])
	try:
		semantic_mappings = parse_semantic_mappings_from_md(md_file)
	except ValueError:
		semantic_mappings = [
			SemanticMapping(
				utterance=m.slang,
				intent="点单-黑话映射",
				slots="source=hard_mapping_auto",
				candidate_category="未分类",
				candidate_skus=[m.sku],
				primary_sku=m.sku,
			)
			for m in state.get("hard_mappings", [])
		]
	return {"semantic_mappings": semantic_mappings}


def _node_build_hard_chunks(state: IngestState) -> IngestState:
	hard_chunks = build_hard_mapping_chunks(
		state["hard_mappings"],
		Path(state["md_file"]).name,
	)
	return {"hard_chunks": hard_chunks}


def _node_build_semantic_chunks(state: IngestState) -> IngestState:
	semantic_chunks = build_semantic_chunks(
		state["semantic_mappings"],
		state["hard_mappings"],
		Path(state["md_file"]).name,
	)
	return {"semantic_chunks": semantic_chunks}


def _node_upsert_md(state: IngestState) -> IngestState:
	h_ids, h_docs, h_metas = state["hard_chunks"]
	s_ids, s_docs, s_metas = state["semantic_chunks"]

	if state.get("dry_run", False):
		result = {
			"source": state["md_file"],
			"input_type": state.get("input_type", "md"),
			"dry_run": True,
			"hard_rows": len(state["hard_mappings"]),
			"semantic_rows": len(state["semantic_mappings"]),
			"upsert_hard_chunks": len(h_ids),
			"upsert_semantic_chunks": len(s_ids),
			"deleted_hard_chunks": 0,
			"deleted_semantic_chunks": 0,
			"collection": COLLECTION_NAME,
		}
		return {"result": result}

	client = chromadb.HttpClient(host=state["host"], port=state["port"])
	collection = client.get_or_create_collection(name=COLLECTION_NAME)

	deleted_hard, upsert_hard = _upsert_source_chunks(
		collection,
		h_ids,
		h_docs,
		h_metas,
		source_file=f"{Path(state['md_file']).name}::hard",
	)
	deleted_sem, upsert_sem = _upsert_source_chunks(
		collection,
		s_ids,
		s_docs,
		s_metas,
		source_file=f"{Path(state['md_file']).name}::semantic",
	)

	result = {
		"source": state["md_file"],
		"input_type": state.get("input_type", "md"),
		"dry_run": False,
		"hard_rows": len(state["hard_mappings"]),
		"semantic_rows": len(state["semantic_mappings"]),
		"upsert_hard_chunks": upsert_hard,
		"upsert_semantic_chunks": upsert_sem,
		"deleted_hard_chunks": deleted_hard,
		"deleted_semantic_chunks": deleted_sem,
		"collection": COLLECTION_NAME,
	}
	return {"result": result}


def _build_md_ingest_graph():
	graph = StateGraph(IngestState)
	graph.add_node("route_input", lambda _: {})
	graph.add_node("use_md_input", _node_use_md_input)
	graph.add_node("convert_doc_to_md", _node_convert_doc_to_md)
	graph.add_node("parse_md_hard", _node_parse_md_hard)
	graph.add_node("parse_md_semantic", _node_parse_md_semantic)
	graph.add_node("build_hard_chunks", _node_build_hard_chunks)
	graph.add_node("build_semantic_chunks", _node_build_semantic_chunks)
	graph.add_node("upsert_md", _node_upsert_md)

	graph.set_entry_point("route_input")
	graph.add_conditional_edges(
		"route_input",
		_route_input_type,
		{
			"md": "use_md_input",
			"doc": "convert_doc_to_md",
		},
	)
	graph.add_edge("use_md_input", "parse_md_hard")
	graph.add_edge("convert_doc_to_md", "parse_md_hard")
	graph.add_edge("parse_md_hard", "parse_md_semantic")
	graph.add_edge("parse_md_semantic", "build_hard_chunks")
	graph.add_edge("build_hard_chunks", "build_semantic_chunks")
	graph.add_edge("build_semantic_chunks", "upsert_md")
	graph.add_edge("upsert_md", END)
	return graph.compile()


def sync_to_chroma(host: str = "localhost", port: int = 8000) -> None:
	sync_md_graph_to_chroma(host=host, port=port, md_file=ZHENGXING_MD_FILE)


def sync_md_graph_to_chroma(
	host: str = "localhost",
	port: int = 8000,
	dry_run: bool = False,
	input_file: Path | None = None,
	md_file: Path = ZHENGXING_MD_FILE,
) -> None:
	graph = _build_md_ingest_graph()
	selected_input = input_file or md_file
	result_state = graph.invoke(
		{
			"host": host,
			"port": port,
			"dry_run": dry_run,
			"input_file": str(selected_input),
			"md_file": str(md_file),
		}
	)
	result = result_state["result"]
	print(f"dry_run={result.get('dry_run', False)}")
	print(f"input_type={result.get('input_type', 'md')}")
	print(f"md_source={result['source']}")
	print(f"hard_rows={result['hard_rows']}")
	print(f"semantic_rows={result['semantic_rows']}")
	print(f"upsert_hard_chunks={result['upsert_hard_chunks']}")
	print(f"upsert_semantic_chunks={result['upsert_semantic_chunks']}")
	print(f"deleted_hard_chunks={result['deleted_hard_chunks']}")
	print(f"deleted_semantic_chunks={result['deleted_semantic_chunks']}")
	print(f"collection={result['collection']}")


def sync_doc_graph_to_chroma(
	host: str = "localhost",
	port: int = 8000,
	doc_file: Path = DATA_FILE,
	dry_run: bool = False,
) -> None:
	sync_md_graph_to_chroma(
		host=host,
		port=port,
		dry_run=dry_run,
		input_file=doc_file,
	)


def sync_semantic_md_to_chroma(
	host: str = "localhost",
	port: int = 8000,
	md_file: Path = ZHENGXING_MD_FILE,
	dry_run: bool = False,
) -> None:
	sync_md_graph_to_chroma(
		host=host,
		port=port,
		dry_run=dry_run,
		md_file=md_file,
	)


def query_demo(text: str, k: int = 5, host: str = "localhost", port: int = 8000) -> None:
	client = chromadb.HttpClient(host=host, port=port)
	collection = client.get_collection(name=COLLECTION_NAME)
	result = collection.query(query_texts=[text], n_results=k)
	docs = result.get("documents", [[]])[0]
	metas = result.get("metadatas", [[]])[0]
	print(f"query={text}")
	for i, (doc, meta) in enumerate(zip(docs, metas), start=1):
		print(f"[{i}] {meta.get('doc_type')} | sku={meta.get('sku')} | product={meta.get('product_name')}")
		print(doc)
		print("-" * 60)


def main() -> None:
	parser = argparse.ArgumentParser(description="Generate slang SKU source doc and sync to ChromaDB")
	sub = parser.add_subparsers(dest="cmd", required=True)

	ingest = sub.add_parser("ingest", help="Read source doc, chunk, and sync update to ChromaDB")
	ingest.add_argument("--host", default="localhost")
	ingest.add_argument("--port", type=int, default=8000)
	ingest.add_argument("--dry-run", action="store_true", help="Parse and summarize only, do not write to vector DB")
	ingest.add_argument("--input-file", default=None, help="Auto-routed input file (.md or .docx)")
	ingest.add_argument("--md-file", default=str(ZHENGXING_MD_FILE))

	ingest_doc = sub.add_parser("ingest-doc", help="Docx compatibility ingest via LangGraph")
	ingest_doc.add_argument("--host", default="localhost")
	ingest_doc.add_argument("--port", type=int, default=8000)
	ingest_doc.add_argument("--dry-run", action="store_true", help="Parse and summarize only, do not write to vector DB")
	ingest_doc.add_argument("--doc-file", default=str(DATA_FILE))

	ingest_semantic = sub.add_parser(
		"ingest-semantic",
		help="Parse semantic fuzzy mapping from zhengxing.md and sync to ChromaDB",
	)
	ingest_semantic.add_argument("--host", default="localhost")
	ingest_semantic.add_argument("--port", type=int, default=8000)
	ingest_semantic.add_argument("--dry-run", action="store_true", help="Parse and summarize only, do not write to vector DB")
	ingest_semantic.add_argument("--md-file", default=str(ZHENGXING_MD_FILE))

	query = sub.add_parser("query", help="Quick query demo")
	query.add_argument("text")
	query.add_argument("-k", type=int, default=5)
	query.add_argument("--host", default="localhost")
	query.add_argument("--port", type=int, default=8000)

	args = parser.parse_args()
	if args.cmd == "ingest":
		input_file = Path(args.input_file) if args.input_file else Path(args.md_file)
		sync_md_graph_to_chroma(
			host=args.host,
			port=args.port,
			dry_run=args.dry_run,
			input_file=input_file,
			md_file=Path(args.md_file),
		)
		return
	if args.cmd == "ingest-doc":
		sync_doc_graph_to_chroma(
			host=args.host,
			port=args.port,
			doc_file=Path(args.doc_file),
			dry_run=args.dry_run,
		)
		return
	if args.cmd == "ingest-semantic":
		sync_semantic_md_to_chroma(
			host=args.host,
			port=args.port,
			md_file=Path(args.md_file),
			dry_run=args.dry_run,
		)
		return
	if args.cmd == "query":
		query_demo(text=args.text, k=args.k, host=args.host, port=args.port)


if __name__ == "__main__":
	main()
