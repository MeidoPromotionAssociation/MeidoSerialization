#!/usr/bin/env python3
"""生成 GitHub Release 正文

Generate the GitHub Release body

读取 .github/scripts/release-template.md，把模板中的占位符替换为本次发布的实际内容：
{{CHANGES}} 替换为两个 tag 之间的提交列表（带 commit 链接），{{TAG}} / {{PREV_TAG}}
替换为版本号，{{COMPARE_URL}} 替换为 changelog 对比链接

Reads .github/scripts/release-template.md and replaces the placeholders with the content of
this release: {{CHANGES}} becomes the commit list between two tags (with commit links),
{{TAG}} / {{PREV_TAG}} become the version numbers, and {{COMPARE_URL}} becomes the
changelog comparison link

本地预览：
  python3 .github/scripts/release_notes.py --tag v2.3.0
"""

import argparse
import re
import subprocess
import sys

# git log 输出中分隔各字段和各记录的控制字符，避免与提交信息本身的字符冲突
# Control characters separating fields and records in git log output, so they cannot
# collide with characters inside the commit message itself
FIELD_SEP = "\x1f"
RECORD_SEP = "\x1e"

# Markdown 链接文本中需要转义的字符，否则含方括号的提交信息会破坏链接语法
# Characters to escape inside Markdown link text, otherwise a commit subject containing
# square brackets would break the link syntax
LINK_TEXT_ESCAPE = re.compile(r"([\[\]])")

# 从远端地址里取出 owner/repo，同时兼容 ssh 和 https 两种写法
# Extract owner/repo from the remote URL, accepting both the ssh and the https form
REMOTE_PATTERN = re.compile(r"[:/]([^/:]+/[^/]+?)(?:\.git)?$")


def git(*args: str) -> str:
    """执行 git 命令并返回去除首尾空白的标准输出，失败时返回空字符串

    Run a git command and return its stripped stdout, or an empty string on failure
    """
    result = subprocess.run(
        ["git", *args],
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
    )
    if result.returncode != 0:
        return ""
    return result.stdout.strip()


def resolve_repo() -> str:
    """从 origin 远端地址推导 owner/repo，推导不出来时返回空字符串

    Derive owner/repo from the origin remote URL, returning an empty string when it cannot be found
    """
    match = REMOTE_PATTERN.search(git("config", "--get", "remote.origin.url"))
    if not match:
        return ""
    return match.group(1)


def resolve_prev_tag(tag: str) -> str:
    """查找 tag 之前最近的一个 tag，首次发布时返回空字符串

    Find the tag closest before the given tag, returning an empty string on a first release
    """
    return git("describe", "--tags", "--abbrev=0", f"{tag}^")


def build_changes(repo: str, tag: str, prev_tag: str) -> str:
    """生成提交列表，每行一个 Markdown 链接，按时间从旧到新排列

    Build the commit list as one Markdown link per line, ordered from oldest to newest
    """
    revision_range = f"{prev_tag}..{tag}" if prev_tag else tag
    raw = git(
        "log",
        revision_range,
        "--reverse",
        "--no-merges",
        f"--format=%H{FIELD_SEP}%s{RECORD_SEP}",
    )

    lines = []
    for record in raw.split(RECORD_SEP):
        record = record.strip()
        if not record:
            continue
        commit, _, subject = record.partition(FIELD_SEP)
        subject = LINK_TEXT_ESCAPE.sub(r"\\\1", subject.strip())
        lines.append(f"- [{subject}](https://github.com/{repo}/commit/{commit})")

    if not lines:
        return "- No code changes"
    return "\n".join(lines)


def build_compare_url(repo: str, tag: str, prev_tag: str) -> str:
    """生成 changelog 链接，首次发布时退化为提交历史链接

    Build the changelog link, falling back to the commit history link on a first release
    """
    if prev_tag:
        return f"https://github.com/{repo}/compare/{prev_tag}...{tag}"
    return f"https://github.com/{repo}/commits/{tag}"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tag", required=True, help="本次发布的 tag / tag being released")
    parser.add_argument("--prev-tag", default="", help="上一个 tag，留空则自动推导 / previous tag, auto-detected when empty")
    parser.add_argument("--repo", default="", help="owner/repo，留空则从 origin 推导 / owner/repo, derived from origin when empty")
    parser.add_argument("--template", default=".github/scripts/release-template.md", help="模板路径 / template path")
    parser.add_argument("--output", default="", help="输出文件，留空则写到标准输出 / output file, stdout when empty")
    args = parser.parse_args()

    repo = args.repo or resolve_repo()
    if not repo:
        print("cannot determine owner/repo from origin, pass --repo explicitly", file=sys.stderr)
        return 1

    prev_tag = args.prev_tag or resolve_prev_tag(args.tag)

    try:
        with open(args.template, encoding="utf-8") as f:
            template = f.read()
    except OSError as err:
        print(f"failed to read template: {err}", file=sys.stderr)
        return 1

    body = template
    for placeholder, value in (
        ("{{CHANGES}}", build_changes(repo, args.tag, prev_tag)),
        ("{{COMPARE_URL}}", build_compare_url(repo, args.tag, prev_tag)),
        ("{{PREV_TAG}}", prev_tag),
        ("{{TAG}}", args.tag),
    ):
        body = body.replace(placeholder, value)

    if args.output:
        with open(args.output, "w", encoding="utf-8", newline="\n") as f:
            f.write(body)
        print(f"wrote {args.output} (tag={args.tag}, prev_tag={prev_tag or 'none'})", file=sys.stderr)
    else:
        # 本地预览时终端可能不是 UTF-8（例如 Windows 默认的 GBK），强制按 UTF-8 输出
        # The terminal may not be UTF-8 when previewing locally (such as GBK on Windows),
        # so force UTF-8 output
        sys.stdout.buffer.write(body.encode("utf-8"))

    return 0


if __name__ == "__main__":
    sys.exit(main())
