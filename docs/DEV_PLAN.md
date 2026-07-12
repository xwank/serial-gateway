# 串口 TCP 共享网关 — 开发方案

## 1. 项目概述

在 Windows PC 上运行串口 TCP 网关服务，通过固定连接的 USB Hub 管理多块 CH341 开发板，将每个 Hub 槽位映射为 `IP:TCP端口`，支持局域网内多个 SecureCRT 客户端同时连接同一路串口（广播输出、写队列串行化）。

### 1.1 目标

- Server：Windows 10 / Windows 10 Server / Windows 11（amd64，单 exe）
- 实验硬件：6 口有源 USB Hub + CH341 开发板
- 客户端：SecureCRT / PuTTY（Telnet 或 Raw TCP）
- 不刷 CH341 序列号，靠 **USB Location Path + Hub 固定插位** 识别设备

### 1.2 不做

- 断线重连历史缓冲（scrollback）
- USB over IP
- Web 终端（第一期）
- OpenOCD 二进制烧录保证

---

## 2. 架构

```
[6口有源 Hub] ──USB──► [Windows Server]
                              │
                    serial-gateway.exe
                              │
              ┌───────────────┼───────────────┐
         TCP :2001       :2002  ...      :2006
              │               │
        SecureCRT ×N     SecureCRT ×N
        (多人同看同写)
```

### 2.1 模块划分（C 风格）

| 模块 | 路径 | 职责 |
|------|------|------|
| config | internal/config | YAML 配置加载与校验 |
| log | internal/log | 统一日志 |
| device | internal/device | Windows COM 枚举 + USB Location |
| slot | internal/slot | 槽位生命周期 |
| session | internal/session | 多客户端、广播、写队列 |
| serialio | internal/serialio | 串口读写 |
| tcpio | internal/tcpio | TCP 连接（内嵌于 slot） |
| telnet | internal/telnet | Telnet IAC 过滤 |

### 2.2 数据流

```
TCP Client ──► conn_reader ──► session.EnqueueWrite ──► write_queue
                                                              │
                                                              ▼
                                                         serial_writer ──► COM

COM ──► serial_reader ──► session.Broadcast ──► 所有 TCP Client
```

**铁律：每个 slot 仅一个 goroutine 写入 COM。**

---

## 3. 设备识别（无序列号）

### 3.1 匹配规则

1. `match_location` 后缀匹配 USB Location Path（主）
2. `hub_anchor.location_contains` 过滤其他 Hub
3. COM 号仅用于显示，不作为绑定依据

### 3.2 硬件纪律

- Hub 永远插 PC 同一 USB 口
- 板子 i 永远插 Hub 物理口 i
- 关闭 USB 选择性暂停
- Server IP 建议 DHCP 保留

### 3.3 校准流程

1. Hub 接到最终 USB 口
2. 一块板子轮流插 Hub 口 1~6，每次运行 `scanports.exe`
3. 记录各口 `LocationPath`，写入 `configs/gateway.yaml`
4. `scanports.exe --verify -c configs/gateway.yaml` 校验

---

## 4. 配置文件

见 `configs/gateway.yaml`。每槽位：

- `id`：1~6
- `tcp_port`：2001~2006
- `match_location`：如 `USB(5)#USB(1)`
- `baud` / `data_bits` / `parity` / `stop_bits`

---

## 5. 多人共享会话

- **下行**：串口读到数据 → 广播给所有在线客户端
- **上行**：各客户端写入 → 进入队列 → 单线程顺序写串口
- **断线**：移除客户端，不保留 buff
- **模式**：字符流（byte stream），非行缓冲

---

## 6. SecureCRT

- 协议：Telnet（端口 2001~2006）
- Gateway 过滤 Telnet IAC 协商字节
- 不回显（由终端/设备处理）

---

## 7. Windows 兼容

| 组件 | 方案 |
|------|------|
| 串口 | go.bug.st/serial |
| 设备枚举 | SetupAPI / registry（Win10+ 通用） |
| 构建 | `GOOS=windows GOARCH=amd64` |
| 防火墙 | scripts/install-firewall.ps1 |
| 服务（可选） | kardianos/service |

---

## 8. 开发阶段

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0 | scanports + Location 枚举 | 完成 |
| P1 | 单槽单客户端 TCP↔串口透传 | 完成 |
| P2 | 多客户端 session 共享 | 完成 |
| P3 | 6 槽 yaml + verify + 热插拔 | 完成（待实机校准） |
| P4 | Telnet 过滤、日志、三版 Windows 测试 | 待做 |
| P5 | Windows 服务 + 安装脚本 | 待做 |
| P6 | 单 exe Web 管理界面（scanports + gateway 合一） | 完成 |

## 11. 单 exe 管理界面

主程序 `bin/serial-gateway.exe` 内置 Web UI（浏览器打开）：

- 扫描本机 IP、扫描 USB 串口
- 界面分配 TCP 端口、描述
- 保存 `gateway.yaml`（exe 同目录）
- 启动/停止网关
- 日志 `gateway.log`（1MB 轮转）

无需 CGO，复制 exe 即可在另一台 Windows 上运行。

---

## 9. 目录结构

```
serial-gateway/
├── cmd/
│   ├── gateway/          # 主服务
│   └── scanports/        # 端口扫描/校准工具
├── internal/
│   ├── config/
│   ├── device/
│   ├── log/
│   ├── session/
│   ├── serialio/
│   ├── slot/
│   └── telnet/
├── configs/
│   └── gateway.yaml
├── docs/
│   └── DEV_PLAN.md
├── scripts/
│   └── install-firewall.ps1
├── go.mod
└── README.md
```

---

## 10. 风险

| 风险 | 对策 |
|------|------|
| Hub 换 USB 口 | 重新校准；启动 verify |
| 板子插错口 | 标签 + verify |
| 廉价 Hub 口序异常 | 以 Location 实测为准 |
| Server 防火墙 | 安装脚本放行 2001-2006 |

---

*文档版本：v1.0 | 更新：2026-07-09*
