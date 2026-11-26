# 小米 BE7000 MosDNS

适用于小米 BE7000 的 DNS 转发器，基于 [IrineSistiana/mosdns](https://github.com/IrineSistiana/mosdns) 的 v5 源码，导入基线为 [`9cfb7ce`](https://github.com/IrineSistiana/mosdns/commit/9cfb7ce985599c087cb7ccfb1531d0c0f4021242)。

## 功能

- 为 `qualcommax/ipq95xx` 固件构建 Linux ARM64 可执行文件，使用 ARMv8.0 指令基线并静态链接 musl。
- 启用延迟缓存时，同一缓存键已有后台刷新任务的请求会直接跳过重复刷新和查询上下文复制。

## 快速开始

从 [GitHub Actions](https://github.com/lyleee0083/xiaomi-be7000-mosdns/actions) 或 [Releases](https://github.com/lyleee0083/xiaomi-be7000-mosdns/releases) 下载 `mosdns-xiaomi-be7000-0.0.1.zip`。解压后，在目标 Linux ARM64 环境验证二进制版本。

```sh
./mosdns version
```

## 配置

归档包含二进制、说明和许可证。MosDNS 继续读取启动参数指定的配置文件。插件、上游、监听地址和运行目录沿用原有配置方式，详见 [MosDNS Wiki](https://irine-sistiana.gitbook.io/mosdns-wiki/)。

标记为 `ordinary_cache` 和 `node_cache` 的缓存插件支持将 `args.size` 设为 `1` 至 `65535`。该值是缓存记录总数的上限。省略时使用原默认值 `1024`。显式填写越界值会导致配置加载失败。

## 构建与测试

在仓库根目录使用 Go 1.26、Python 3.10 或更新版本，并把 [Zig 官方发行包](https://ziglang.org/download/)中的 `zig` 加到 PATH。CI 使用 Zig 0.15.2。版本记录在 [VERSION](VERSION)，构建包输出到 `release/`，文件名为 `mosdns-xiaomi-be7000-<版本>.zip`。

```sh
go test ./...
python release.py
```

## 发布

测试工作流提供 BE7000 构建产物。发布标签须匹配 [VERSION](VERSION)，例如版本 `0.0.1` 对应标签 `v0.0.1`。

## 许可证

[GNU GPL v3 或更高版本](LICENSE)。
