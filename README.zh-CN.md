<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/public/appicon.png" />
    <img src="frontend/public/appicon-light.png" alt="lich" width="88" height="88" />
  </picture>
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <strong>简体中文</strong></p>
  <p><a href="https://omartelo.github.io/lich/index.zh-CN.html"><strong>omartelo.github.io/lich</strong></a></p>
  <p><strong>为你已在用的编码智能体打造的终端优先 ADE（智能体开发环境）。</strong></p>
  <p>
    打开你的项目，在真实终端里运行 Claude Code、Codex、opencode 这样的智能体，
    并把 git —— worktree、diff 和 Pull Request —— 一并留在视野里，无需离开窗口。
    单个静态 Go 二进制文件，没有 Electron：在 Linux、Windows 和 macOS 上，
    界面都在 lich 自带的内嵌 Chromium 里打开。
  </p>
  <p>
    <a href="https://github.com/omartelo/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/omartelo/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" />
    <img alt="Shell" src="https://img.shields.io/badge/shell-embedded%20Chromium%20(CEF)-4285F4?logo=googlechrome&logoColor=white" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <img src="docs/media/session.png" alt="同一面墙上并排的四个 Claude Code 会话，各自待在自己的 git worktree 里 —— 侧栏逐个列出它们的分支和 diff 徽标，底栏显示模型、套餐额度与分支" width="900" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

> 本文档译自英文 [README.md](README.md)。英文版本是唯一的事实来源；若两者出现分歧，以英文为准。

## 为什么选 lich

用 lich，你可以：

- **用你已经有的智能体。** [Claude Code](https://www.anthropic.com/claude-code)、
  [Codex](https://github.com/openai/codex)、Antigravity、
  [opencode](https://github.com/sst/opencode)、oh-my-pi、
  [Crush](https://github.com/charmbracelet/crush)、
  [Cursor CLI](https://cursor.com/docs/cli) 和
  [Kiro CLI](https://kiro.dev/docs/cli/) 在这里都以同样的方式运行。把 lich
  指向各自的二进制文件，这只需一次，之后选一个默认的，或者逐个会话单独指定。而
  lich 能从一个会话里*读*回什么，则因智能体而异，
  [各智能体支持到什么程度](#各智能体支持到什么程度)是那张表。
- **留住一个真正的终端。** 由 PTY 支撑的 shell，每个项目可以开好几个，在 GPU 上渲染
  —— 滚动缓冲区可以搜索，还能挺过整页刷新。给其中一个设一个入口命令 —— `lazygit`、
  `k9s`、`pnpm dev` —— 它每次启动都会直接进到那个工具里，在 Linux、macOS 和 Windows
  上都一样：不会先加载 shell rc 或 PowerShell profile，所以对你管用的入口命令，对你分享给
  的人也一样管用；把开发服务器写进 `.lich/run-worktree.sh`，每个检出目录都会为它多出一张
  **Run** 卡片，跑在 lich 为那个 worktree 预留的端口上。底栏跟随 `cd` 并标明分支
  —— 对于 Claude Code、Codex 或 Kiro CLI 会话，还有模型和已占用的上下文窗口；对于
  Claude Code、Codex、Antigravity、Cursor CLI 和 OpenCode Go 订阅，还有你的套餐额度还剩
  多少；如果你要求的话，还有会话花了多少钱，只要该智能体记下了这个数。
- **让一个会话为另一个干活。** 把任务交给另一张卡片，答案由它自己的智能体写回来，两端
  各跑着什么都不影响：智能体通过启动时交予的工具够到其他会话 —— Claude Code 和 Codex
  走 MCP —— 其余的随插件获得。这整套能力同时也是任何 shell 里的 `lich` 命令，`--json`
  也在其中，所以脚本不需要智能体参与也能驱动一个会话
  （[`docs/cli.md`](docs/cli.md)）。
- **一次看住好几个。** 把任意会话摆到你正在看的那个旁边，凑成一面墙，pane 怎么排由
  lich 自己算 —— 依据有几个，以及窗口还剩多少地方：八个在超宽屏上是横四竖二，在笔记本
  上则横着摆两个。一个把活派出去的会话，一次点击就能围着它把墙搭起来。每面墙都有名字，
  一个项目想留多少面都行，每面墙在侧栏里各占一块，可以折叠、重命名，也可以拆掉。
- **免去配置地分出一个 worktree。** 从任意基础分支开一个，lich 会把你被 gitignore 掉的
  `.env*` 文件播种进去，派给它一个既不与其他检出目录重合、也没被机器上任何进程占用的
  开发服务器端口，并在智能体启动前跑一遍按项目配置的初始化脚本。也可以从当前检出目录
  尚未提交的改动开一个，这些改动会一起带过去，未跟踪的文件也包括在内。
- **让会话在沙箱里运行。** 一个智能体可以开在操作系统的沙箱里：一个只装着该 provider
  自身状态的空白 home，机器的其余部分只读，只有它被打开时所在的那个检出目录可写。你的
  ssh 密钥、你的云凭据以及磁盘上其他所有仓库，在里面根本不存在 —— 这正是放手让智能体
  跳过权限确认的那一头的配重。Linux 用 bubblewrap，macOS 用 `sandbox-exec`；它不是
  针对恶意代码的边界，[`docs/ceilings.md`](docs/ceilings.md) 写明了它挡不住什么。
- **在读 diff 的地方审查它。** 一个 CodeMirror 面板在实时文件树旁展示工作区改动，可以
  按需折叠只改了空白的部分，改动过的图片或 PDF 会显示改动前后。右键选中的内容就能针对
  这些行写评论；整批评论会作为一条 prompt 粘贴进会话，但不发送。它的 Code 标签页可以浏览
  检出目录，并搜索其中文件的文本。
- **在这里把 Pull Request 送到终点。** 列出仓库开启的 Pull Request，把其中一个检出到
  它专属的 worktree，然后阅读 diff、内联审查并合并它 —— 用基础分支实际接受的方式。
  在还没有 Pull Request 的分支上，**Create with agent** 会让该分支的会话去开一个，
  你按下回车之前什么都不会发送。

除此之外：[主题](docs/themes.md)可以按 JSON 导入或从 git 仓库安装，`Ctrl`/`Cmd`+`K`
命令面板既能按名字也能按对话里说过的内容跳转，某个会话在等你输入时会发一条桌面通知，
`lich rage` / `lich doctor` 则留给它完全起不来的时候。

项目处于活跃开发中：bug 和功能需求请提到
[Issues](https://github.com/omartelo/lich/issues)，每个版本改了什么见
[CHANGELOG.md](CHANGELOG.md)。

## 各智能体支持到什么程度

每个智能体的启动方式都一样：在真实终端里拉起，按会话 id 恢复，用同一套沙箱隔离，也用
同一套办法去找你的其他会话。不一样的是 lich 能从一个会话里**读**回什么，因为它读的是
各家 CLI 自己写下来的东西，而没有两家写下的是同一批。

| 你能看到什么 | Claude Code | Codex | Antigravity | opencode | oh-my-pi | Crush | Cursor CLI | Kiro CLI |
| --- | :-: | :-: | :-: | :-: | :-: | :-: | :-: | :-: |
| 底栏里的上下文窗口 | 有 | 有 | 无 | 无 | 无 | 无 | 无 | 有 |
| 底栏里的花费 | 有 | 有 | 无 | 有 | 有 | 有 | 无 | 按点数计 |
| 套餐额度还剩多少 | 有 | 有 | 有 | 仅 OpenCode Go | 无 | 无 | 有 | 无 |
| 被套餐用量上限打断的一轮，在上限重置后自动接着做 | 有 | 有 | 无 | 由其自身重试 | 由其自身重试 | 无 | 无 | 无 |
| 一轮进行中转圈，结束后成环 | 有 | 有 | 有 | 有 | 有 | 无 | 无 | 有 |
| 智能体卡在等你时响铃 | 有 | 有 | 无 | 有 | 无 | 无 | 无 | 无 |
| 侧栏按状态筛选：Waiting、Running、Unread | 有 | 有 | 无 Waiting | 有 | 无 Waiting | 无，总在 Idle | 无，总在 Idle | 无 Waiting |
| 一轮跑着时不让机器进入空闲休眠 | 有 | 有 | 有 | 有 | 有 | 无 | 无 | 有 |
| Review 页的 **Last turn**，带智能体自己的收尾话 | 有 | 有 | 有 | 有 | 有 | 无 | 无 | 有 |
| 从命令面板搜索对话里说过的内容 | 有 | 有 | 有 | 有 | 有 | 有 | 无 | 有 |
| 在命令面板的 History 页恢复在 lich 之外开始的对话 | 有 | 有 | 有 | 有 | 有 | Linux；macOS 和 Windows 未测 | 有 | 有 |
| 把一段对话分叉到新的 worktree | 有 | 有 | 无 | 有 | 无 | 无 | 无 | 无 |
| 按指定的推理强度打开会话 | 有 | 有 | 有 | 无 | 有 | 无 | 写在模型名里 | 有 |
| 以 ultracode 模式打开会话 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 用 `lich control` 或智能体操控运行中的会话：发提示、停止、换模型、推理强度、斜杠命令 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 其他会话发来的消息直接送达，不经终端键入 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 智能体启动的子智能体作为 lich 会话运行，有自己的卡片，在发起会话的检出目录中工作，交回完整报告后自动关闭 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 向运行中的会话问一个旁支问题，不打断它（`lich ask`） | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 上下文、花费和套餐用量取 CLI 自己量出的数 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 终端状态栏显示本会话欠谁一个回复、在等谁，以及待收取的回复 | 有 | 无 | 无 | 无 | 无 | 无 | 无 | 无 |
| 子代理或工作流步骤私下委派任务 | 有 | 有 | 有 | 有 | 有 | 有 | 有 | 有 |
| 恢复的会话保留原来的模型和推理强度 | 仅模型 | 有 | 无 | 模型 | 有 | 无 | 模型 | 模型；推理强度未测 |
| 打断等待结果后，结果仍留着等你收取 | 有 | 有 | 有 | 有 | 无 | 有 | 有 | 有 |

Crush 和 Cursor CLI 既不报告一轮的开始，也不报告结束，上面有四行是因此一起掉的：没有
东西去开启那个窗口，卡片的转圈、响铃、Review 页的上一轮，以及让机器不休眠的那次占用，
都无从谈起。Review 页会在会话上直接说明这一点，而不是让你自己去发现那个切换从来没出现
过。Kiro CLI 按点数而不是按美元计费，所以那个数字只有它自己的底栏能看到。
Antigravity 的套餐额度靠直接问 `agy` 读出（需要 1.1.11 或更新；更旧的版本回答时会花掉
一轮，所以 lich 不去问它）；opencode 只有在你登录了 OpenCode Go 订阅时才有，否则它用的是
你自己的 API key；Cursor CLI 读的是 `cursor-agent login` 写下的登录，从不读 Cursor 编辑器的。
oh-my-pi、Crush 和 Kiro CLI 没有 lich 能读到的套餐。
操控运行中的会话需要 Claude Code 2.1.280 或更新版本，加上 lich-plugin 0.15.0：它经由
Claude Code 的 mod 实现，其他 CLI 内部都没有能让 lich 递交命令的东西在运行。同一个
mod 配上 lich-plugin 0.16.0，会报告 Claude Code 对自己量出的数；其他 CLI 的底栏，以及没有
它的 Claude Code 会话，显示的是 lich 从 CLI 写下的记录里读出来的数。
History 页的 **Outside lich** 分组列出在你某个项目的检出目录里跑过、而 lich 没有持有的对话。
lich 打开过、又被你彻底关掉的会话不会出现在里面，只有 0.62.0 之前关掉的例外：lich 靠启动时
给 Claude Code 起的名字认出自己的对话，其他七个 CLI 没有写下任何能区分的东西，所以它们的
这类对话会在那里出现一次。

每一处缺口都是刻意的，没有一处是 lich 藏起了 CLI 已经报告的东西，
[`docs/ceilings.md`](docs/ceilings.md) 写着每一处背后量到了什么。

## 安装

一行命令 —— 识别你的发行版、校验校验和，并通过你的包管理器安装原生软件包及其依赖：

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| 平台 | 安装方式 | 运行时依赖 |
| --- | --- | --- |
| **Linux** | 上面的 `install.sh`，或 AUR 的 [`lich-bin`](https://aur.archlinux.org/packages/lich-bin)（`yay -S lich-bin`） | `zenity`，以及随软件包附带的窗口所需的 glibc 2.34 或更新版本（Debian 12、Ubuntu 22.04、RHEL 9 及以上） |
| **macOS** *(实验性)* | `brew install --cask omartelo/tap/lich` | 无需任何东西 —— Apple Silicon 和 Intel 上窗口都随应用一起附带 |
| **Windows** | 从 [Releases](https://github.com/omartelo/lich/releases) 下载安装程序或便携版 zip，或使用 Scoop：`scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json` | 无需任何东西，每种方式都自带窗口 |

手动的分发版软件包、Linux 压缩包和 Windows 便携版 zip 见 [INSTALL.md](INSTALL.md)。macOS 和 Windows 的
二进制文件未签名 —— 在公证/签名做好之前，Gatekeeper 和 SmartScreen 会发出警告。用
Homebrew 安装可以绕开 Gatekeeper 的提示；从 Releases 页面下载的二进制文件则需要手动
清除隔离标记。在 macOS 上，cask 会把 `Lich.app` 装进 `/Applications`，lich 因此有了
自己的图标，运行期间 Dock 里显示的也是它。从旧的 formula 升级需要先 `brew uninstall lich`，
[INSTALL.md](INSTALL.md) 里写了这一点。

## 快速上手

1. **安装**并启动 `lich`。
2. **打开一个项目** —— 标签栏里的 `+` 会列出你最近关掉的项目，也能调起系统的文件夹
   选择器；指向一个 git 仓库即可。
3. **把 lich 指向你的智能体** —— 首次启动会列出在你机器上找到的智能体；之后可以在
   设置 › Providers 里设置各自的二进制文件路径，并选定默认用哪一个。项目可以沿用
   这个默认，也可以在同一屏里选另一个 provider。
4. **开一个会话** —— *New Session* 会在项目里启动一个跑着你的智能体的终端。每个
   checkout 的标题栏也有一个 `+` 菜单，可以在那个 checkout 里直接打开任意已启用的
   provider 或一个纯终端；点标题栏本身则会折叠或展开它下面的会话。
5. **分出一个 worktree**（可选）—— 从任意基础分支创建一个；lich 会为它播种文件，并把你
   带进一个新的会话。

## 配置

- **智能体** —— 在设置 › Providers 里为每个 provider 设定二进制文件路径，并选定所有
  项目的默认。那里当前项目的那一行可以覆盖这个选择；**Clear** 会移除这层覆盖，之后默认
  的变化便会自动流过来。
  lich 能读到套餐的 provider，其一节的开头是你的套餐还剩多少。底栏显示什么，在设置 ›
  Appearance 里拖动它的各项来设定；费用读数一开始是隐藏的，因为只有当你按 token 计费时
  这个数字才有意义。
- **Worktree** —— 项目仓库里的 `.lich/setup-worktree.sh` 会在新 worktree 的终端里
  先于智能体运行；New worktree 对话框会展示这个脚本，若仓库没有则给出检测到的建议。
  `.worktreeinclude` 文件用来调整哪些被 gitignore 的文件会被复制过去。
- **会话钩子** —— 在设置里装上 [lich 插件](https://github.com/omartelo/lich-plugin)
  之后，会话会给自己的卡片起标题，并在它写入文件的那一刻刷新 git。
- **稳定性承诺** —— 哪些部分可以放心写脚本依赖、哪些可能在任何版本里变动，写在
  [docs/stability.md](docs/stability.md)（英文）。

## 隐私与更新

一切都跑在你自己的机器上。没有账号，不用登录，没有遥测：后端是一个带 token 鉴权的本地
回环监听器。lich 只会为下面这些事主动联网，除此之外没有别的：

- **更新**：启动时以及每小时向 GitHub Releases 查询一次版本，启动时还会读取 GitHub 上
  lich 插件的发布列表。
- **套餐用量**：各 provider 自己的用量接口（Anthropic、OpenAI、Cursor、OpenCode Go），
  用会话的 CLI 已有的登录，最多每五分钟一次。Antigravity 的套餐直接问 `agy` 本身。
- **价格**：GitHub 上 LiteLLM 公开的价格表，只在内置价格表不认识某个模型时取一次。

Windows 安装程序装的版本会就地更新；用 Homebrew、Scoop、AUR 或 Linux 软件包装的，
则通过各自的包管理器更新，lich 会把命令递给你。
设置 › 帮助会在你把日志附到 bug 报告之前，说明日志文件里都带了什么 —— 路径、项目名和
分支名、你的 gh 登录名，绝不包含会话 token；`lich rage` 会把这份报告收进一个压缩包，
而不上传其中任何内容。

## 从源码构建

纯 Go 后端（Go 1.27，`CGO_ENABLED=0`），通过带 token 鉴权的本地回环监听器（HTTP RPC +
WebSocket）伺服内嵌的 React 18 / TypeScript / Vite 前端。终端是 xterm.js 配 WebGL
插件；代码和 diff 界面是 CodeMirror 6。Chromium 外壳有一份决策记录：
[`docs/chromium-shell.md`](docs/chromium-shell.md)。前置条件是 **Go 1.27.0+**、
**Node + pnpm** 和 **[Task](https://taskfile.dev)** —— Go 二进制文件不需要 C 工具链。
窗口（`shell/`，基于 CEF 的 Rust）另外需要 Rust 工具链、CMake 和 Ninja，Linux 上还要
Chromium 的开发库，Windows 上还要 MSVC；[CONTRIBUTING.md](CONTRIBUTING.md) 列出了它们。

```bash
task dev      # 热重载开发模式（Vite 跑在 :9245）
task build    # 生产二进制文件 -> bin/lich
task run      # 构建并运行
task test     # Go 与前端测试套件
```

在本地打包一个 Linux 发行版本（需要 [nfpm](https://nfpm.goreleaser.com/)）：

```bash
task package   # bin/ 下生成 .deb + .rpm + Arch .pkg.tar.zst
```

在 lich 已经跑着的八个之外再加一个智能体 CLI，是唯一一处会落到两个仓库、十几个文件里
的改动：[`docs/adding-a-provider.md`](docs/adding-a-provider.md) 就是那张地图。

## 赞助

lich 由一个人编写和维护。赞助为投入其中的时间买单，也让这个项目保持独立：这个应用没有
付费版本，将来也不会有。

[**成为赞助者**](https://github.com/sponsors/omartelo)

<!-- sponsor-names: monthly sponsors go here -->

### 支持者

<!-- backers: one-time supporters go here -->

暂时还没有。

## 许可

[AGPL-3.0-only](LICENSE) © 2026 omartelo

lich 是自由软件：你可以在 GNU Affero 通用公共许可证第 3 版的条款下使用、研究、修改和
再分发它。任何被分发或通过网络提供服务的衍生作品，都必须以同一许可证发布。截至 v0.9.0
（含）的发行版本仍为 MIT 许可。
