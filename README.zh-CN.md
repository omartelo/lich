<div align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/public/appicon.png" />
    <img src="frontend/public/appicon-light.png" alt="lich" width="88" height="88" />
  </picture>
  <h1>lich</h1>
  <p><a href="README.md">English</a> · <strong>简体中文</strong></p>
  <p><a href="https://omartelo.github.io/lich/index.zh-CN.html"><strong>omartelo.github.io/lich</strong></a></p>
  <p><strong>为 AI 编码智能体打造的终端优先工作台。</strong></p>
  <p>
    打开你的项目，在真实终端里运行 Claude Code、Codex、opencode 这样的智能体，
    并把 git —— worktree、diff 和 Pull Request —— 一并留在视野里，无需离开窗口。
    单个静态 Go 二进制文件，没有 Electron：界面在你系统自带的 Chromium 系浏览器中以
    <code>--app</code> 模式打开。
  </p>
  <p>
    <a href="https://github.com/omartelo/lich/releases"><img alt="Release" src="https://img.shields.io/github/v/release/omartelo/lich?color=4285F4&label=release" /></a>
    <img alt="Go" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" />
    <img alt="Shell" src="https://img.shields.io/badge/shell-Chromium%20--app-4285F4?logo=googlechrome&logoColor=white" />
    <img alt="Platform" src="https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-333" />
    <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-AGPL--3.0-blue" /></a>
    <a href="https://github.com/sponsors/omartelo"><img alt="Sponsor" src="https://img.shields.io/github/sponsors/omartelo?color=ea4aaa&logo=githubsponsors&label=sponsors" /></a>
  </p>
  <img src="docs/media/session.png" alt="标签栏上的四个项目，侧栏里的五个会话 —— 各自带着 worktree、分支和 diff 徽标 —— 与此同时一个 Claude Code 会话正在终端里工作，底栏显示着模型和上下文圆环" width="900" />
  <!-- sponsor-logos: company logos go here, between the screenshot and Why lich -->
</div>

> 本文档译自英文 [README.md](README.md)。英文版本是唯一的事实来源；若两者出现分歧，以英文为准。

## 为什么选 lich

用 lich，你可以：

- **用你已经有的智能体。** [Claude Code](https://www.anthropic.com/claude-code)、
  [Codex](https://github.com/openai/codex)、[opencode](https://github.com/sst/opencode)、
  oh-my-pi 和 [Crush](https://github.com/charmbracelet/crush) 都是一等公民。把 lich
  指向各自的二进制文件，这只需一次，之后选一个默认的，或者逐个会话单独指定。
- **留住一个真正的终端。** 由 PTY 支撑的 shell，每个项目可以开好几个，在 GPU 上渲染
  —— 滚动缓冲区可以搜索，还能挺过整页刷新。底栏跟随 `cd` 并标明分支 —— 对于 Claude
  会话，还有模型、已占用的上下文窗口，以及 —— 如果你要求的话 —— 这个会话花了多少钱。
- **让一个会话为另一个干活。** 把任务交给另一张卡片，答案由它自己的智能体写回来，两端
  各跑着什么都不影响：智能体通过启动时交予的工具够到其他会话 —— Claude Code 和 Codex
  走 MCP —— 其余的随插件获得。这整套能力同时也是任何 shell 里的 `lich` 命令，`--json`
  也在其中，所以脚本不需要智能体参与也能驱动一个会话
  （[`docs/cli.md`](docs/cli.md)）。
- **免去配置地分出一个 worktree。** 从任意基础分支开一个，lich 会把你被 gitignore 掉的
  `.env*` 文件播种进去，派给它一个既不与其他检出目录重合、也没被机器上任何进程占用的
  开发服务器端口，并在智能体启动前跑一遍按项目配置的初始化脚本。
- **在读 diff 的地方审查它。** 一个 CodeMirror 面板在实时文件树旁展示工作区改动。右键
  选中的内容就能针对这些行写评论；整批评论会作为一条 prompt 粘贴进会话，但不发送。
- **在这里把 Pull Request 送到终点。** 列出仓库开启的 Pull Request，把其中一个检出到
  它专属的 worktree，然后阅读 diff、内联审查并合并它 —— 用基础分支实际接受的方式。

除此之外：[主题](docs/themes.md)可以按 JSON 导入或从 git 仓库安装，`Ctrl`/`Cmd`+`K`
命令面板既能按名字也能按对话里说过的内容跳转，某个会话在等你输入时会发一条桌面通知，
`lich rage` / `lich doctor` 则留给它完全起不来的时候。

项目处于活跃开发中：bug 和功能需求请提到
[Issues](https://github.com/omartelo/lich/issues)，每个版本改了什么见
[CHANGELOG.md](CHANGELOG.md)。

## 安装

一行命令 —— 识别你的发行版、校验校验和，并通过你的包管理器安装原生软件包及其依赖：

```bash
curl -fsSL https://raw.githubusercontent.com/omartelo/lich/main/install.sh | sh
```

| 平台 | 安装方式 | 运行时依赖 |
| --- | --- | --- |
| **Linux** | 上面的 `install.sh`，或 AUR 的 [`lich-bin`](https://aur.archlinux.org/packages/lich-bin)（`yay -S lich-bin`） | `PATH` 上有 chromium / google-chrome / brave，外加 `zenity` |
| **macOS** *(实验性)* | `brew install --cask omartelo/tap/lich` | `/Applications` 里有 Chrome / Chromium / Edge / Brave |
| **Windows** *(实验性)* | 从 [Releases](https://github.com/omartelo/lich/releases) 下载安装程序 | Chrome / Edge / Brave |

手动的分发版软件包和静态二进制文件见 [INSTALL.md](INSTALL.md)。macOS 和 Windows 的
二进制文件未签名 —— 在公证/签名做好之前，Gatekeeper 和 SmartScreen 会发出警告。用
Homebrew 安装可以绕开 Gatekeeper 的提示；从 Releases 页面下载的二进制文件则需要手动
清除隔离标记。在 macOS 上，cask 会把 `Lich.app` 装进 `/Applications`，lich 因此有了
自己的图标；而它运行期间，Dock 里显示的仍是持有那个窗口的浏览器。从旧的 formula 升级
需要先 `brew uninstall lich`，[INSTALL.md](INSTALL.md) 里写了这一点。

## 快速上手

1. **安装**并启动 `lich`。
2. **打开一个项目** —— 标签栏里的 `+` 会列出你最近关掉的项目，也能调起系统的文件夹
   选择器；指向一个 git 仓库即可。
3. **把 lich 指向你的智能体** —— 首次启动会列出在你机器上找到的智能体；之后可以在
   设置 › Providers 里设置各自的二进制文件路径，并改掉新会话默认用哪一个。
4. **开一个会话** —— *New Session* 会在项目里启动一个跑着你的智能体的终端。
5. **分出一个 worktree**（可选）—— 从任意基础分支创建一个；lich 会为它播种文件，并把你
   带进一个新的会话。

## 配置

- **智能体** —— 在设置里为每个 provider 设定二进制文件路径，并选定新会话默认用哪一个。
  Claude Code 那一节还管着底栏的上下文圆环和费用读数 —— 费用默认关闭，因为只有当你按
  token 计费时这个数字才有意义。
- **Worktree** —— 按项目配置的初始化脚本（设置 › Worktree）会在新 worktree 的终端里
  先于智能体运行；`.worktreeinclude` 文件用来调整哪些被 gitignore 的文件会被复制过去。
- **版本控制** —— 一个项目可以指定 `gh` 以哪个 GitHub 账号运行（设置 › Version
  Control），用于只有你其中一个账号看得见的仓库。它管的是 lich 从 GitHub 读到什么，
  而不是 git 推送时用谁的身份。
- **外观与快捷键** —— 主题、字体和组合键都在设置里；你选定的主题持久化在工作区数据库里，
  其余 UI 偏好以 `lich.*` 为键持久化在 `localStorage`（位于 lich 的 Chromium 配置目录
  `~/.config/lich/chromium-profile`），导入的主题则以 JSON 存在 `<config-dir>/lich/themes` 下。
- **工作区** —— 项目和会话持久化在 SQLite 里，路径为 `<config-dir>/lich/lich.db`。
  关闭一个会话并不会删除它。
- **会话钩子** —— 在设置里装上 [lich 插件](https://github.com/omartelo/lich-plugin)
  之后，会话会给自己的卡片起标题，并在它写入文件的那一刻刷新 git。

## 隐私与更新

一切都跑在你自己的机器上。没有账号，不用登录，没有遥测 —— 后端是一个带 token 鉴权的
本地回环监听器，除了更新检查之外没有任何东西离开 `localhost`：启动时以及每小时向 GitHub
Releases 发一次版本查询。更新在 Windows/macOS 上就地应用，在 Arch 上通过 AUR 进行。
设置 › 帮助会在你把日志附到 bug 报告之前，说明日志文件里都带了什么 —— 路径、项目名和
分支名、你的 gh 登录名，绝不包含会话 token；`lich rage` 会把这份报告收进一个压缩包，
而不上传其中任何内容。

## 从源码构建

纯 Go 后端（Go 1.25，`CGO_ENABLED=0`），通过带 token 鉴权的本地回环监听器（HTTP RPC +
WebSocket）伺服内嵌的 React 18 / TypeScript / Vite 前端。终端是 xterm.js 配 WebGL
插件；代码和 diff 界面是 CodeMirror 6。Chromium 外壳有一份决策记录：
[`docs/chromium-shell.md`](docs/chromium-shell.md)。前置条件是 **Go 1.25+**、
**Node + pnpm** 和 **[Task](https://taskfile.dev)** —— 不需要 C 工具链，也不需要
系统开发库。

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
