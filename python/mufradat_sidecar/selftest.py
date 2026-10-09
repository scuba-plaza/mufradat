import importlib
import json
import sys

from .engine import Engine, versions

PROBE = {"key": "selftest", "field": "example", "tokens": ["ذهب", "الولد", "إلى", "المدرسة"], "context": True}


def run():
    readings = []
    errors = []
    engine = Engine(log=lambda message: print(message, file=sys.stderr))
    engine.check([PROBE], readings.append, lambda key, message: errors.append(message), lambda done, total: None)
    problems = list(errors)
    if len(readings) != 1:
        problems.append(f"expected one reading, got {len(readings)}")
    else:
        reading = readings[0]
        if not all(reading["analyses"]):
            problems.append("CAMeL returned no analysis for a common word")
        if not reading["catt"]:
            problems.append("CATT returned no vowelled reading")
        if not reading["bert"]:
            problems.append("the CAMeL BERT disambiguator returned nothing")
    print(json.dumps({"ok": not problems, "problems": problems, "versions": versions(), "reading": readings[:1]}, ensure_ascii=False))
    return 0 if not problems else 1


MODULES = ("numpy", "onnxruntime", "torch", "transformers", "camel_tools", "catt_tashkeel")


def probe():
    problems = []
    for name in MODULES:
        try:
            importlib.import_module(name)
        except Exception as exc:
            problems.append(f"{name}: {type(exc).__name__}: {exc}")
    print(json.dumps({"ok": not problems, "problems": problems, "versions": versions()}, ensure_ascii=False))
    return 0 if not problems else 1
