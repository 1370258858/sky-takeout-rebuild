import asyncio
import test_asyncio2_run
import time
async def main():
    task1 = asyncio.create_task(
        test_asyncio2_run.sayHi(1,"Hello") )
    print(f"started at {time.strftime('%X')}")


    task2 = asyncio.create_task(
        test_asyncio2_run.sayHi(2, 'world'))
    # create_task 注册完成
    # task1 和task2 同步执行
    await task1
    await task2
    print(f"started at {time.strftime('%X')}")

asyncio.run(main())


# output
# started at 13:17:50
# world
# Hello
# started at 13:18:00