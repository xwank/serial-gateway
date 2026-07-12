# Hub 槽位校准指南

## 前提

- 使用 **有源** USB Hub
- Hub 插入 PC **后置 USB 口**，记下是哪个口（贴标签）
- 以后不再更换 Hub 的 PC 端 USB 口

## 单块板子轮流校准（推荐）

1. 启动 Hub，暂不插开发板
2. 运行：

   ```powershell
   .\bin\scanports.exe
   ```

3. 将开发板插入 Hub **物理口 1**，再运行 `scanports.exe`，记录输出中的 `LOCATION` 列
4. 将同一块板移到 Hub 口 2、3 … 6，重复记录
5. 将 6 条 `LOCATION` 填入 `configs/gateway.yaml` 各 slot 的 `match_location`

示例：

```yaml
slots:
  - id: 1
    tcp_port: 2001
    match_location: "USB(5)#USB(1)"
```

6. 6 块板分别插回 Hub 口 1~6，验证：

   ```powershell
   .\bin\scanports.exe --verify -c configs\gateway.yaml
   ```

## 验证通过标准

- 6 个槽位各匹配到唯一设备
- 无重复匹配
- 无未匹配槽位（板子均在线时）

## 常见问题

**Q：换了一个 PC USB 口怎么办？**  
A：Location 会变，需重新校准。

**Q：设备管理器里 COM 号变了？**  
A：正常。网关按 Location 找 COM，不依赖 COM 号。

**Q：两个槽位匹配到同一设备？**  
A：检查 `match_location` 是否过于模糊，应使用更长的路径片段。
