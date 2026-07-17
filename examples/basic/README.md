# Basic example

A tiny project you can check with pypls right now. It has three files:

- `shapes.py`: a module with two type problems pypls catches.
- `clean.py`: a module with none, to show what a clean file looks like.
- `pyproject.toml`: a `[tool.pypls]` section with an `exclude` list.

## Run it

From this directory:

```
pypls check .
```

pypls checks both Python files and reports what it finds in `shapes.py`:

```
shapes.py:6:12: error: unsupported operand type(s) for +: int and str
shapes.py:10:22: warning: value of type str is not assignable to int
checked 2 file(s), found 2 problem(s)
```

Because one of the problems is an `error`, the command exits with status `1`,
which is what makes pypls useful in continuous integration.

## The problems

`shapes.py:6` adds a string to an `int`, which Python does not allow, so pypls
reports an `error`:

```python
def scale(width: int, factor: int) -> int:
    return width * factor + "px"
```

`shapes.py:10` annotates a name as `int` but assigns a string, which is a
`warning`:

```python
default_width: int = "wide"
```

## The clean file

Check `clean.py` on its own and pypls stays silent:

```
pypls check clean.py
```

```
checked 1 file(s), no problems found
```

## The configuration

`pyproject.toml` holds the pypls settings for this example:

```toml
[tool.pypls]
python = "python3.12"
exclude = ["build", "*.gen.py"]
```

`exclude` lists files and directories pypls skips. There is nothing to exclude
in this small project, but the entry shows where the setting goes. See the
[tutorial](../../docs/tutorial.md) for `exclude` in action.
