import asyncio
import time
async def sayHi(delay,message):
    # async.sleep 是包中的专用方法
    await asyncio.sleep(delay)
    # print(message, flush=True)
    print(message)



async def main():
    print(f"started at {time.strftime('%X')}")
    await sayHi(1,"Hello2")
    await sayHi(2,"World2")
    print(f"started at {time.strftime('%X')}")


if __name__ == "__main__":
    asyncio.run(main())
# main()
# 报错RuntimeWarning: coroutine 'main' was never awaited

# started at 13:17:50
# world
# Hello
# started at 13:18:01
