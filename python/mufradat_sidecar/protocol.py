import json


def emit(out, event):
    out.write(json.dumps(event, ensure_ascii=False, separators=(",", ":")) + "\n")
    out.flush()


def requests(stream):
    for line in stream:
        line = line.strip()
        if not line:
            continue
        yield json.loads(line)
