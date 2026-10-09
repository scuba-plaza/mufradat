from . import PROTOCOL, VERSION
from .protocol import emit, requests


def serve(engine, stdin, stdout, versions):
    emit(stdout, {"type": "hello", "protocol": PROTOCOL, "version": VERSION, "versions": versions})
    for request in requests(stdin):
        kind = request.get("type")
        rid = request.get("id")
        if kind == "shutdown":
            return 0
        if kind != "check":
            emit(stdout, {"type": "error", "id": rid, "message": f"unknown request type {kind!r}"})
            continue
        handle_check(engine, stdout, rid, request.get("items", []))
    return 0


def handle_check(engine, stdout, rid, items):
    def on_reading(reading):
        emit(stdout, {"type": "reading", "id": rid, **reading})

    def on_error(key, message):
        emit(stdout, {"type": "item_error", "id": rid, "key": key, "message": message})

    def on_progress(done, total):
        emit(stdout, {"type": "progress", "id": rid, "done": done, "total": total})

    def on_status(message):
        emit(stdout, {"type": "status", "id": rid, "message": message})

    engine.log = on_status
    try:
        engine.check(items, on_reading, on_error, on_progress)
    except Exception as exc:
        emit(stdout, {"type": "error", "id": rid, "message": f"{type(exc).__name__}: {exc}"})
        return
    emit(stdout, {"type": "done", "id": rid})
