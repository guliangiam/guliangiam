#!/usr/bin/env python3
"""check-token-parity.py — 设计令牌「生成物零差异」门禁

`web/src/styles.css` 是 `--gl-*` 设计令牌的唯一事实源（设计报告 28.1）；antd 读不到
CSS 自定义属性，只能在 `web/src/app/theme/brand-palette.ts` 里吃字面量。该镜像文件
**由生成器产出**（`scripts/gen_brand_palette.py`），因此漂移只有两种成因：镜像陈旧，
或有人手改了镜像。两者都会被本门禁发现。

判定方式（不再逐值比对两份手写文件，而是比对生成结果与磁盘文件）：
  用当前 `styles.css` 重新渲染镜像，与磁盘上的 `brand-palette.ts` 做逐字节比较；
  不一致即漂移，修复方式恒定为「改 styles.css 后重新生成」。

Usage: python3 scripts/check-token-parity.py
Exit code 0 = 零差异，1 = 镜像陈旧或被手改。

输入：web/src/styles.css、web/src/app/theme/brand-palette.ts
输出：PASS: 零差异；FAIL: brand-palette.ts 陈旧或被手改
--check 行为：无 --check（本身就是 --check 模式的薄包装，调用 gen_brand_palette.check()）
消费方：verify-docs.sh(RUN_CODE_CHECKS=1)、CI
"""

import sys

from gen_brand_palette import check

if __name__ == "__main__":
    sys.exit(check())
