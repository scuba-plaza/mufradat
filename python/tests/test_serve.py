import io
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from mufradat_sidecar.serve import serve


class FakeEngine:
    def __init__(self, fail_on=None, crash=False):
        self.log = lambda message: None
        self.fail_on = fail_on
        self.crash = crash

    def check(self, items, on_reading, on_error, on_progress):
        self.log("loading")
        if self.crash:
            raise RuntimeError("models are missing")
        done = 0
        for item in items:
            if item["key"] == self.fail_on:
                on_error(item["key"], "boom")
            else:
                on_reading({"key": item["key"], "analyses": [["x"] for _ in item["tokens"]], "catt": None, "bert": None})
            done += 1
            on_progress(done, len(items))


def run(engine, *lines):
    stdin = io.StringIO("".join(json.dumps(line) + "\n" for line in lines))
    stdout = io.StringIO()
    code = serve(engine, stdin, stdout, {"python": "3.x"})
    return code, [json.loads(line) for line in stdout.getvalue().splitlines()]


def test_hello_comes_first_and_shutdown_ends_the_loop():
    code, events = run(FakeEngine(), {"type": "shutdown"})
    assert code == 0
    assert events == [{"type": "hello", "protocol": 1, "version": "1", "versions": {"python": "3.x"}}]


def test_check_streams_status_readings_progress_and_done():
    items = [{"key": "a", "tokens": ["w1", "w2"]}, {"key": "b", "tokens": ["w3"]}]
    code, events = run(FakeEngine(), {"type": "check", "id": "7", "items": items}, {"type": "shutdown"})
    assert code == 0
    kinds = [(e["type"], e.get("key"), e.get("done")) for e in events[1:]]
    assert kinds == [("status", None, None), ("reading", "a", None), ("progress", None, 1), ("reading", "b", None), ("progress", None, 2), ("done", None, None)]
    assert all(e["id"] == "7" for e in events[1:])
    assert events[2]["analyses"] == [["x"], ["x"]]


def test_one_bad_item_does_not_stop_the_others():
    items = [{"key": "a", "tokens": ["w"]}, {"key": "b", "tokens": ["w"]}]
    _, events = run(FakeEngine(fail_on="a"), {"type": "check", "id": "1", "items": items})
    types = [e["type"] for e in events[1:]]
    assert "item_error" in types and types.count("reading") == 1 and types[-1] == "done"
    assert [e for e in events if e["type"] == "item_error"][0]["key"] == "a"


def test_engine_crash_is_reported_and_serving_continues():
    _, events = run(FakeEngine(crash=True), {"type": "check", "id": "1", "items": []}, {"type": "check", "id": "2", "items": []})
    errors = [e for e in events if e["type"] == "error"]
    assert [e["id"] for e in errors] == ["1", "2"]
    assert "models are missing" in errors[0]["message"]
    assert not any(e["type"] == "done" for e in events)


def test_unknown_request_is_an_error_not_a_crash():
    _, events = run(FakeEngine(), {"type": "dance", "id": "9"})
    assert events[1] == {"type": "error", "id": "9", "message": "unknown request type 'dance'"}


def test_stdin_closing_ends_the_loop():
    code, events = run(FakeEngine())
    assert code == 0 and len(events) == 1
