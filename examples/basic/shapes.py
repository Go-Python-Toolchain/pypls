"""A small module with a couple of type problems for pypls to catch."""


def scale(width: int, factor: int) -> int:
    # factor is an int, so adding a string to it is not allowed.
    return width * factor + "px"


# The annotation says int, but the value is a string.
default_width: int = "wide"
