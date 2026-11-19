"""python -m collector 入口：转发到 batch CLI。"""

from .cli import main

if __name__ == "__main__":
    raise SystemExit(main())
