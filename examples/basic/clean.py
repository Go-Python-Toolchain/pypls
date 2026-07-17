"""A clean module. pypls reports nothing here."""


def area(width: int, height: int) -> int:
    return width * height


def label(name: str, count: int) -> str:
    return name + " x" + str(count)


total: int = area(3, 4)
