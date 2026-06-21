import asyncio

async def nested():
    return 42

async def main():
    # 调度 nested() 与 "main()" 并发运行。
    task = asyncio.create_task(nested())

    # 现在可以通过 "task" 来取消 "nested()"，
    # 也可以等待 "task" 直到它被完成：
    await task

asyncio.run(main())