import argparse
import re
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path


DATABASE_CALLS = re.compile(r"\.(?:First|Find|Raw|Exec)\(")
SAVE_CALL = re.compile(r"\.Save\(")
GO_FUNC = re.compile(r"\bgo\s+func\b")
RECOVER = re.compile(r"\brecover\(\)")
RESERVED_WORDS = ("key", "order", "desc", "group")
SQL_LITERAL = re.compile(r"`([^`]+)`")
MODEL_ANNOTATION = "删除策略:"
ANNOTATED_MODELS = {
    "Account", "Application", "Permission", "PermissionResource", "RAccountRole",
    "Resource", "Role", "RRolePermission",
}


@dataclass
class Violation:
    category: str
    path: Path
    line: int
    detail: str


def production_go_files(root: Path):
    for path in root.rglob("*.go"):
        if "_test.go" not in path.name:
            yield path


def check_save(path: Path, lines):
    for number, line in enumerate(lines, 1):
        if SAVE_CALL.search(line):
            yield Violation("save", path, number, "整对象落库：.Save(")


def check_database_call_in_loop(path: Path, lines):
    for number, line in enumerate(lines, 1):
        stripped = line.strip()
        if not stripped.startswith("for ") and stripped != "for" or "{" not in stripped:
            continue
        indent = len(line) - len(line.lstrip("\t"))
        depth = 0
        for position in range(number, len(lines) + 1):
            current = lines[position - 1]
            depth += current.count("{") - current.count("}")
            if position > number and depth <= 0:
                break
            if position > number and DATABASE_CALLS.search(current):
                yield Violation("loop", path, position, "疑似循环内数据库调用")


def check_goroutine_recover(path: Path, lines):
    for number, line in enumerate(lines):
        if GO_FUNC.search(line):
            nearby = "\n".join(lines[number : min(number + 12, len(lines))])
            if not RECOVER.search(nearby):
                yield Violation("goroutine", path, number + 1, "go func 附近缺少 recover")


def check_reserved_words(path: Path, lines):
    for number, line in enumerate(lines, 1):
        if not DATABASE_CALLS.search(line):
            continue
        for word in RESERVED_WORDS:
            quoted = re.compile(rf"`\s*{word}\s*`", re.IGNORECASE)
            unquoted = re.compile(rf"(?<![`\w]){word}(?![`\w])", re.IGNORECASE)
            if unquoted.search(line) and not quoted.search(line):
                yield Violation("reserved-word", path, number, f"SQL 保留字 {word} 需要反引号")


def check_model_annotations(path: Path, lines):
    if path.parent.name != "model":
        return
    for number, line in enumerate(lines):
        match = re.match(r"type\s+(\w+)\s+struct\s+\{", line)
        if not match or match.group(1) not in ANNOTATED_MODELS:
            continue
        comment_start = number - 1
        has_annotation = False
        while comment_start >= 0 and lines[comment_start].lstrip().startswith("//"):
            if MODEL_ANNOTATION in lines[comment_start]:
                has_annotation = True
                break
            comment_start -= 1
        if not has_annotation:
            yield Violation("model-annotation", path, number + 1, "模型 struct 缺少删除策略标注")


def scan(root: Path):
    violations = []
    for path in production_go_files(root):
        lines = path.read_text(encoding="utf-8").splitlines()
        for check in (check_save, check_database_call_in_loop, check_goroutine_recover, check_reserved_words, check_model_annotations):
            violations.extend(check(path, lines))
    return violations


def report(violations):
    for item in violations:
        print(f"{item.category}:{item.path}:{item.line}: {item.detail}")
    if violations:
        print(f"constitution check failed: {len(violations)} violation(s)")
    else:
        print("constitution check passed: 0 violations")


def self_test():
    with tempfile.TemporaryDirectory() as temp_dir:
        root = Path(temp_dir)
        model_dir = root / "model"
        model_dir.mkdir()
        source = model_dir / "sample.go"
        source.write_text(
            "package main\n\n"
            "type Role struct {\n"
            "\tId int64 `gorm:\"column:id\"`\n"
            "}\n\n"
            "func run(db *DB) {\n"
            "\tfor _, item := range []int{1} {\n"
            "\t\t_ = item\n"
            "\t\tdb.Find(&Sample{})\n"
            "\t}\n"
            "\tgo func() { _ = db }()\n"
            "\tdb.Raw(\"SELECT * FROM t_resource WHERE key = ?\", 1)\n"
            "\tdb.Save(&Sample{})\n"
            "\t_ = db\n"
            "}\n",
            encoding="utf-8",
        )
        violations = scan(root)
        categories = {item.category for item in violations}
        expected = {"save", "loop", "goroutine", "reserved-word", "model-annotation"}
        missing = expected - categories
        if missing:
            raise AssertionError(f"self-test missed violations: {sorted(missing)}")
        if not any(item.line == 3 for item in violations if item.category == "model-annotation"):
            raise AssertionError("self-test did not report exact model annotation line")
        print("constitution checker self-test passed")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("target", nargs="?")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return 0
    if not args.target:
        parser.error("target is required unless --self-test is used")
    violations = scan(Path(args.target))
    report(violations)
    return 1 if violations else 0


if __name__ == "__main__":
    sys.exit(main())
