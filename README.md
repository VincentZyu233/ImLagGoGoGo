# ImLagGoGoGo 🎮

<div align="center">

**极轻量、防吞字、CS2 战术沉浸风的死后全自动狡辩工具 (Go 重构版)**

[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-Windows%20x64%20%7C%20Linux%20(WIP)-blue)](https://github.com/VincentZyuApps/ImLagGoGoGo)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

</div>

---

## 📌 项目灵感与致谢 (Upstream Reference)

本项目是基于著名开源项目 [**cneicy/ImLag**](https://github.com/cneicy/ImLag)（*“CS2死后全自动狡辩工具”*）理念的全新 Go 语言现代重构版本。

致敬原版作者 **Eicy** 及上游社区贡献者们带来的绝妙创意！在继承上游“死后自动发借口”核心精髓的基础上，本项目针对日常开黑与竞技对局中的诸多痛点进行了全方位深度重构与升级。

---

## ✨ 核心特性

- 🚀 **极低资源开销**：基于 Go 原生开发，常驻后台内存仅 **~15MB**，对 CS2 竞技帧率完全零干扰。
- 🛡️ **Unicode 直接键入（彻底告别输入法候选框）**：底层采用 Windows `SendInput` + `KEYEVENTF_UNICODE` 码点直投，无论当前是搜狗拼音还是微软拼音，绝不会切出拼音选词框吞字。
- 🎯 **CS2 Panorama 原生战术沉浸风 UI**：精心还原 CS2 游戏内原生设置页面的碳黑网格质感与青蓝微光设计，就像把游戏内嵌菜单搬到了桌面。
- 🎴 **洗牌池抽样 (Shuffle Deck)**：独创无放回随机洗牌算法，一整套词库借口全部说完前绝不连发重复词，拒绝当“复读机”。
- ⏱️ **555ms 死亡拟真延迟缓冲**：死后智能微停顿 555ms，避开游戏死亡特写与按键惯性，体感逼真不吞键。
- 🔀 **全自动 / 纯手动 / 二者兼有 (Both) 三模支持**：死后自动发 + 手动按键绑定随时发，共享同一套词库池。
- 🔌 **智能 GSI 端口冲突避让**：默认 4000 端口被占时，自动顺延探测（4001 ~ 4999）并全自动重写 CS2 配置，用户完全零感知。
- 📂 **Steam 库全盘自动寻路 + 手动文件选择器兜底**：全自动递归 Steam 多硬盘游戏库部署 GSI，亦支持通过系统弹窗手动指定 CS2 路径。
- 📝 **文本词库自动热监听 + 智能编码嗅探**：支持记事本直接修改 `Messages.txt`，保存瞬间自动无感生效；内置编码嗅探，自动适配 UTF-8 / GBK / ANSI，绝不出现乱码。

---

## 🛠️ 快速开始

### 1. 下载即用
从 [Releases 页面](https://github.com/VincentZyuApps/ImLagGoGoGo/releases) 下载最新的绿色便携压缩包，解压后双击 `imlag.exe` 即可运行。

### 2. 初始配置
首次运行会在同级目录下自动生成 `config.toml` 与 `Messages.txt`：
- **`config.toml`**：填写您的 SteamID64（推荐，前往 [steamid.io](https://steamid.io) 即可查询）或昵称。
- **`Messages.txt`**：一行一条添加你喜欢的借口语句。

点击界面顶部的 **【一键安装 GSI 到 CS2】**，随后启动 CS2 进入对局即可尽情对线！

---

## 📜 许可证

本项目遵循 [MIT License](LICENSE) 开源协议。
