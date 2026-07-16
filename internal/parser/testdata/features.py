"""A module exercising a broad range of Python syntax."""
import os
import sys as system
from collections import OrderedDict, defaultdict
from . import sibling
from ..pkg import thing as renamed


GLOBAL_CONST = 42
names: list[str] = ["a", "b", "c"]
mapping: dict[str, int] = {"one": 1, "two": 2}


@decorator
@namespace.decorator(with_args=True)
class Widget(Base, metaclass=Meta):
    """A widget."""

    count = 0

    def __init__(self, name, *args, size=10, **kwargs):
        self.name = name
        self.size = size
        Widget.count += 1

    async def load(self, source):
        async with open_conn() as conn:
            async for chunk in conn.stream():
                await self.process(chunk)
        return self

    @property
    def label(self) -> str:
        return f"{self.name} ({self.size})"


def compute(values, factor=1.0):
    total = 0
    for i, v in enumerate(values):
        if v is None or v < 0:
            continue
        elif v > 100:
            break
        else:
            total += v * factor
    result = [x ** 2 for x in range(10) if x % 2 == 0]
    squares = {k: k * k for k in values}
    uniq = {x for x in values}
    gen = (y for y in values if y)
    return total, result, squares, uniq, gen


def risky():
    try:
        data = load()
    except (IOError, ValueError) as exc:
        raise RuntimeError("failed") from exc
    except Exception:
        pass
    else:
        return data
    finally:
        cleanup()


lam = lambda a, b=2, *c, **d: a + b
ternary = "yes" if condition else "no"
walrus = [y := 10, y + 1]
sliced = matrix[1:2, ::2, a:b:c]
chained = a < b <= c != d
power = 2 ** 3 ** 2
assert total >= 0, "must be non-negative"
del temporary
global GLOBAL_CONST
