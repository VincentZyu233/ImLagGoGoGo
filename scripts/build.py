#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
ImLagGoGoGo 跨平台统一构建脚本
用于本地一键构建测试与 GitHub Actions CI/CD 复用。
构建产物与测试运行全部隔离在 dist/ 目录下，绝不污染仓库根目录。
"""

import argparse
import os
import platform
import shutil
import subprocess
import sys
import zipfile
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
DIST_DIR = REPO_ROOT / "dist"
ASSETS_DIR = REPO_ROOT / "assets"


def get_default_target() -> str:
    os_name = platform.system().lower()
    if os_name == "windows":
        return "windows-x64"
    elif os_name == "linux":
        return "linux-x64"
    elif os_name == "darwin":
        return "darwin-x64"
    return f"{os_name}-x64"


def clean():
    if DIST_DIR.exists():
        print(f"🧹 正在清理构建目录: {DIST_DIR}")
        shutil.rmtree(DIST_DIR, ignore_errors=True)
    print("✨ 清理完毕！")


def build(target: str, is_debug: bool = False, do_package: bool = False):
    DIST_DIR.mkdir(parents=True, exist_ok=True)

    parts = target.split("-")
    goos = parts[0]
    arch_str = parts[1] if len(parts) > 1 else "x64"
    goarch = "amd64" if arch_str in ("x64", "amd64") else arch_str

    exe_name = "imlag.exe" if goos == "windows" else "imlag"
    output_bin = DIST_DIR / exe_name

    ldflags_list = ["-s", "-w"]
    if goos == "windows" and not is_debug:
        ldflags_list.append("-H=windowsgui")

    ldflags_str = " ".join(ldflags_list)

    cmd = [
        "go", "build",
        f"-ldflags={ldflags_str}",
        "-trimpath",
        "-o", str(output_bin),
        "./cmd/imlag"
    ]

    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"

    print(f"🔨 正在编译 Go 二进制 [{target}]...")
    print(f"   命令: {' '.join(cmd)}")
    result = subprocess.run(cmd, cwd=REPO_ROOT, env=env)
    if result.returncode != 0:
        print(f"❌ 编译失败，退出码: {result.returncode}")
        sys.exit(result.returncode)

    file_size_mb = output_bin.stat().st_size / (1024 * 1024)
    print(f"✅ 编译成功: {output_bin.name} ({file_size_mb:.2f} MB)")

    # 复制模板文件到 dist
    template_files = [
        (ASSETS_DIR / "config.example.toml", DIST_DIR / "config.example.toml"),
        (ASSETS_DIR / "Messages.example.txt", DIST_DIR / "Messages.example.txt"),
        (REPO_ROOT / "README.md", DIST_DIR / "README.md"),
        (REPO_ROOT / "LICENSE", DIST_DIR / "LICENSE"),
    ]

    for src, dst in template_files:
        if src.exists():
            shutil.copy2(src, dst)
            print(f"📋 已复制: {src.name} -> dist/{dst.name}")

    # 如果 dist 下还没有实际使用的 config.toml / Messages.txt，自动生成一份以便本地直接运行
    default_config = DIST_DIR / "config.toml"
    default_corpus = DIST_DIR / "Messages.txt"

    if not default_config.exists() and (ASSETS_DIR / "config.example.toml").exists():
        shutil.copy2(ASSETS_DIR / "config.example.toml", default_config)
        print("💡 已为 dist/ 自动初始化本地测试配置: config.toml")

    if not default_corpus.exists() and (ASSETS_DIR / "Messages.example.txt").exists():
        shutil.copy2(ASSETS_DIR / "Messages.example.txt", default_corpus)
        print("💡 已为 dist/ 自动初始化本地测试词库: Messages.txt")

    # 如果需要打包
    if do_package:
        zip_name = f"ImLagGoGoGo-{target}.zip"
        zip_path = DIST_DIR / zip_name
        print(f"📦 正在打包发布压缩包: {zip_path.name}...")

        files_to_pack = [
            output_bin,
            DIST_DIR / "config.example.toml",
            DIST_DIR / "Messages.example.txt",
            DIST_DIR / "README.md",
            DIST_DIR / "LICENSE",
        ]

        with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
            for f in files_to_pack:
                if f.exists():
                    zf.write(f, arcname=f.name)

        zip_size_mb = zip_path.stat().st_size / (1024 * 1024)
        print(f"🎉 发布包生成成功: {zip_path.name} ({zip_size_mb:.2f} MB)")


def main():
    parser = argparse.ArgumentParser(description="ImLagGoGoGo 构建与打包工具")
    parser.add_argument("--clean", action="store_true", help="清理 dist/ 目录")
    parser.add_argument("--target", default=get_default_target(), help="编译目标平台 (例如 windows-amd64, linux-amd64)")
    parser.add_argument("--debug", action="store_true", help="开启控制台调试窗口 (不添加 -H=windowsgui)")
    parser.add_argument("--package", action="store_true", help="构建完成后打包为 ZIP 发布文件")

    args = parser.parse_args()

    if args.clean:
        clean()
        return

    build(target=args.target, is_debug=args.debug, do_package=args.package)


if __name__ == "__main__":
    main()
