import sys


def main(argv):
    command = argv[1] if len(argv) > 1 else "serve"
    protocol_out = sys.stdout
    sys.stdout = sys.stderr
    if command == "serve":
        from .engine import Engine, versions
        from .serve import serve

        return serve(Engine(), sys.stdin, protocol_out, versions())
    if command == "selftest":
        from .selftest import run

        sys.stdout = protocol_out
        return run()
    if command == "probe":
        from .selftest import probe

        sys.stdout = protocol_out
        return probe()
    print(f"unknown command {command!r}; use serve, selftest or probe", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
