import asyncio
from llm_service.config import load_config
from llm_service.app import create_app
import signal

async def bootstrap() -> None:
    cfg = load_config()
    app = create_app(cfg)
    shutdown_event = asyncio.Event()
    loop= asyncio.get_running_loop()
    def request_shutdown(*_:object) -> None:
        loop.call_soon_threadsafe(shutdown_event.set)

    for shutdown_signal in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(shutdown_signal, request_shutdown)
        except NotImplementedError:
            signal.signal(shutdown_signal, request_shutdown)
    await app.run(shutdown_event=shutdown_event)
def main() -> None:
    asyncio.run(bootstrap())


if __name__ == "__main__":
    main()
