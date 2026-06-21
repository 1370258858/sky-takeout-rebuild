import asyncio
import test_asyncio2_run
import time

async def main():
    async with asyncio.TaskGroup() as tg:
        task1 =   tg.create_task(
            test_asyncio2_run.sayHi(1,"Hello")
        )
         
        task2 =   tg.create_task(
            test_asyncio2_run.sayHi(2,"World")
        )
        print(f"started at {time.strftime('%X')}")

    # 当上下文管理器退出时 await 是隐式执行的。

    print(f"finished at {time.strftime('%X')}")
asyncio.run(main())