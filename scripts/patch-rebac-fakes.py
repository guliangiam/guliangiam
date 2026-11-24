#!/usr/bin/env python3
"""Append Rebac() method + domain import to every custom fake UnitOfWork in all tests.

Handles naming variants:
  - *UnitOfWork (testUnitOfWork, svcTestUnitOfWork, lockoutTestUnitOfWork, ...)
  - *UOW         (appTestUOW, svcTestUOW, enforceUOW, ...)
  - *UnitOfWorkWrap (wrapper types that also need Rebac)

Run from repo root: python3 scripts/patch-rebac-fakes.py

输入：internal/app/**/*_test.go、internal/bootstrap/**/*_test.go
输出：原地修改 fake UnitOfWork 测试文件；打印 modified/skipped 统计
--check 行为：无 --check；脚本为一次性修复工具（非门禁）
消费方：独立手动脚本；不纳入 verify-docs.sh
"""
import glob, re, os, sys

REBAC_IMPORT = '\trebac "github.com/guliangiam/guliangiam/internal/domain/rebac"\n'

files = set()
for pattern in ["internal/app/**/*_test.go", "internal/bootstrap/**/*_test.go"]:
    files.update(glob.glob(pattern, recursive=True))

modified = 0
skipped = 0

for fpath in sorted(files):
    with open(fpath, 'r') as fh:
        content = fh.read()
    original = content

    # 找出所有 fake UoW struct（宽泛匹配）
    fake_names = set()
    # 完整 UnitOfWork 后缀
    for m in re.finditer(r'type (\w+UnitOfWork) struct', content):
        fake_names.add(m.group(1))
    # 缩写 UOW 后缀
    for m in re.finditer(r'type (\w+UOW) struct', content):
        fake_names.add(m.group(1))
    # Uow 后缀（小驼峰）
    for m in re.finditer(r'type (\w+Uow) struct', content):
        fake_names.add(m.group(1))

    if not fake_names:
        skipped += 1
        continue

    # 1. 加 rebac domain import（只需一次）
    if 'internal/domain/rebac' not in content:
        inserted_import = False
        # 优先在 radiusaccountingdomain 后面插
        anchor1 = re.compile(
            r'(\tradiusaccounting\w*domain "github\.com/guliangiam/guliangiam/internal/domain/radiusaccountings"\n)'
        )
        m = anchor1.search(content)
        if m:
            content = content[:m.end()] + REBAC_IMPORT + content[m.end():]
            inserted_import = True

        if not inserted_import:
            # 退而求其次：recordsdomain 前插
            anchor2 = re.compile(
                r'(\trecords\w*domain "github\.com/guliangiam/guliangiam/internal/domain/records"\n)'
            )
            m2 = anchor2.search(content)
            if m2:
                content = content[:m2.start()] + REBAC_IMPORT + content[m2.start():]
                inserted_import = True

        if not inserted_import:
            # 最后：在 import "uow" 后面插（bootstrap fake 里最可能的锚点）
            anchor3 = re.compile(r'(\tappuow "github\.com/guliangiam/guliangiam/internal/app/uow"\n)')
            m3 = anchor3.search(content)
            if m3:
                content = content[:m3.end()] + REBAC_IMPORT + content[m3.end():]

    # 2. 为每个 fake struct 加 Rebac() 方法
    for struct_name in sorted(fake_names):
        if f'func (u {struct_name}) Rebac()' in content:
            continue  # 已有

        method = f'\nfunc (u {struct_name}) Rebac() rebac.Repository {{ return nil }}\n'

        # 优先：在这个 struct 的 WithinTransaction 前插
        m = re.search(rf'func \(u {re.escape(struct_name)}\) WithinTransaction\(', content)
        if m:
            content = content[:m.start()] + method + content[m.start():]
        else:
            # 退而求其次：找这个 struct 最后一个 Repository nil 方法之后
            last_repo = list(re.finditer(
                rf'func \(u {re.escape(struct_name)}\) \w+\(\) [\w./]+\.Repository\s*\{{\s*return nil\s*\}}',
                content
            ))
            if last_repo:
                insert_at = last_repo[-1].end()
                content = content[:insert_at] + method + content[insert_at:]
            else:
                # 最后手段：在这个 struct 定义之后插
                struct_end = content.find(f'type {struct_name} struct')
                if struct_end >= 0:
                    # 找 struct 结束的 }
                    brace = content.find('}', struct_end)
                    if brace >= 0:
                        content = content[:brace+1] + '\n' + method + content[brace+1:]

    if content != original:
        with open(fpath, 'w') as fh:
            fh.write(content)
        modified += 1
        print(f"MODIFIED: {fpath}")

print(f"\nDone. modified={modified} skipped={skipped}")
