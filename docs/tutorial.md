# Tutorial

This walkthrough builds a tiny project, catches real problems with pypls, fixes
them, and then shows strict mode and project configuration. Every command and
every block of output below is real. Follow along and you should see the same
results.

If you have not installed pypls yet, see [Getting Started](getting-started.md).

## 1. Make a project

Create a folder and a file inside it:

```
mkdir greeter
cd greeter
```

Put this in `greetings.py`:

```python
def greet(name: str) -> str:
    return "Hello, " + name


message: str = greet("Ada")
count: int = "three"
```

The `greet` function is fine. The last line is not: `count` is declared as an
`int`, but it is given a string.

## 2. Run your first check

```
pypls check greetings.py
```

pypls finds the mismatch:

```
greetings.py:6:14: warning: value of type str is not assignable to int
checked 1 file(s), found 1 problem(s)
```

The line points at column 14 of line 6, which is exactly where the string value
begins. This is a `warning`: the annotation and the value disagree, but the code
would still run.

## 3. Fix it and check again

Change the last line so the value is a number:

```python
count: int = 3
```

Run the check again:

```
pypls check greetings.py
```

Now pypls is quiet:

```
checked 1 file(s), no problems found
```

## 4. See an error, not just a warning

Some problems are hard errors, not warnings. Add a function that adds a string
to a number. Append this to `greetings.py`:

```python
def scale(width: int, factor: int) -> int:
    return width * factor + "px"
```

Check the file:

```
pypls check greetings.py
```

```
greetings.py:10:12: error: unsupported operand type(s) for +: int and str
checked 1 file(s), found 1 problem(s)
```

This is an `error`, because Python cannot add an `int` and a `str` at all. When
a check finds any error, pypls exits with a non-zero status, which is what makes
it useful in continuous integration. A build step that runs `pypls check` will
fail on this file and pass once it is clean.

Remove that broken function again before moving on, so the file is back to its
clean state.

## 5. Strict mode

By default a warning does not fail the check: pypls prints it but still exits
zero. Sometimes you want warnings to count too. That is what `--strict` is for.

Put a warning back in `greetings.py`:

```python
count: int = "three"
```

Run it normally, then with `--strict`, and compare the exit codes:

```
pypls check greetings.py
```

```
greetings.py:6:14: warning: value of type str is not assignable to int
checked 1 file(s), found 1 problem(s)
```

The command above exits `0`. With strict mode:

```
pypls check --strict greetings.py
```

```
greetings.py:6:14: warning: value of type str is not assignable to int
checked 1 file(s), found 1 problem(s)
```

The output looks the same, but this command exits `1`. Strict mode does not
change what pypls reports, only whether warnings make the check fail. Fix the
line back to `count: int = 3` before the next step.

## 6. Project configuration and exclude

pypls reads settings from the `[tool.pypls]` table of the nearest
`pyproject.toml`, so there is no new config file to learn. One handy setting is
`exclude`, a list of files and directories pypls should skip.

Say you have generated code you do not want checked. Create it:

```
mkdir build
```

Put this in `build/generated.py`:

```python
answer: int = "not a number"
```

With no configuration, pypls checks everything under the current directory:

```
pypls check .
```

```
build/generated.py:1:15: warning: value of type str is not assignable to int
greetings.py:6:14: warning: value of type str is not assignable to int
checked 2 file(s), found 2 problem(s)
```

Now add a `pyproject.toml` next to `greetings.py`:

```toml
[tool.pypls]
exclude = ["build"]
```

Run the same check again:

```
pypls check .
```

```
greetings.py:6:14: warning: value of type str is not assignable to int
checked 1 file(s), found 1 problem(s)
```

The `build` directory is gone from the results, and pypls now reports it checked
only one file. Other settings you can put in the same table are `python` (the
interpreter your project targets), `venv` (your virtual environment), and
`strict` (treat warnings as failures, the same as passing `--strict`). The
`--config` flag lets you point at a specific `pyproject.toml` if you need to.

Note: the last line of `greetings.py` still has the `count: int = "three"`
warning from step 5. That is why the runs above still report it. Fix that line
whenever you want a clean project.

## 7. A note on caching

pypls stores check results under your cache directory, keyed by the content of
each file, so unchanged files are served from the cache instead of being
rechecked. This is invisible in normal use. If you ever want to force a full
recheck, pass `--no-cache` and pypls ignores the cache for that run.

## Next steps

- [Editor setup](editors.md): get these same diagnostics live as you type.
- [examples/basic](../examples/basic): a ready-made version of a project like
  this one.
