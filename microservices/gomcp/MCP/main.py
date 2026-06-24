import asyncio
import sys
from pathlib import Path
import argparse

# 确保 pmcp 和 MCP 根目录在搜索路径中
sys.path.insert(0, str(Path(__file__).parent.parent / "pmcp"))
sys.path.insert(0, str(Path(__file__).parent))

from config.config import resolve_url
from agents.agent_loop import AgentLoop

# 读取命令行参数 session id
parser = argparse.ArgumentParser()
parser.add_argument("--session_id", help="Session ID")

args = parser.parse_args()

async def main():
    # 按需开启对应服务的 URL
    service_urls = {
        "order": resolve_url("order"),
        # "goods":    resolve_url("goods"),
        # "delivery": resolve_url("delivery"),
    }
    # 系统层初始化
    agent = AgentLoop(service_urls)
    # 用户层初始化
    await agent.run(args.session_id)



if __name__ == "__main__":
    asyncio.run(main())
