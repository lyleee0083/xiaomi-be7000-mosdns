# 小米 BE7000 MosDNS

适用于小米 BE7000 的 DNS 转发器，基于 [IrineSistiana/mosdns](https://github.com/IrineSistiana/mosdns) 的 v5 源码，导入基线为 [`9cfb7ce`](https://github.com/IrineSistiana/mosdns/commit/9cfb7ce985599c087cb7ccfb1531d0c0f4021242)。

## 功能

- 为 `qualcommax/ipq95xx` 固件构建 Linux ARM64 可执行文件，使用 ARMv8.0 指令基线并静态链接 musl。
- 启用延迟缓存时，同一缓存键已有后台刷新任务的请求会直接跳过重复刷新和查询上下文复制。
- 主配置采用 JSON。使用内置命令编译域名和 IP 规则，运行时直接加载 MosDNS 二进制规则集。
- UDP/TCP 直接在复用缓冲区中打包。DoH 从 HTTP 响应体读入同一块交给 DNS 解码器的缓冲区，按实际需要扩容。

## 快速开始

从 [GitHub Releases](https://github.com/lyleee0083/xiaomi-be7000-mosdns/releases) 下载对应版本的可执行文件和 `SHA256SUMS.txt`。以下以 `0.0.4` 为例，在目标 Linux ARM64 环境校验文件、添加执行权限并查看版本。

```sh
sha256sum -c SHA256SUMS.txt
chmod +x mosdns-xiaomi-be7000-0.0.4
./mosdns-xiaomi-be7000-0.0.4 version
```

## 配置

默认读取工作目录中的 `config.json`。使用 `start -c /path/to/config.json` 可以指定主配置，`include` 引用的文件同样使用 `.json` 后缀。内置命令 `config gen config.json` 可生成转发模板。

`domain_set` 和 `ip_set` 的 `args.files` 引用由 MosDNS 编译的 `.mdrs` 文件。源文件使用带 `version`、`type` 和 `rules` 的 JSON 对象。版本为 `1`，`type` 为 `domain` 或 `ip`。域名规则支持 `domain:`（含子域）、`full:`、`keyword:` 和 `regexp:`，省略前缀时匹配域名及其子域。IP 规则支持 IPv4、IPv6 地址和 CIDR。二进制文件带有格式版本和规则类型。

在包含 [examples](examples) 目录的位置运行以下命令，可以启动仅监听本机的示例。示例使用保留域名与地址展示域名和响应 IP 匹配。

```sh
./mosdns-xiaomi-be7000-0.0.4 rule-set compile examples/domains.json
./mosdns-xiaomi-be7000-0.0.4 rule-set compile examples/ips.json
./mosdns-xiaomi-be7000-0.0.4 start -c examples/config.json
```

使用 `-o` 可以指定二进制规则的输出路径。配置内引用的相对路径以 MosDNS 工作目录为基准。

标记为 `ordinary_cache` 和 `node_cache` 的缓存插件支持将 `args.size` 设为 `1` 至 `65535`。该值是缓存记录总数的上限。省略时使用原默认值 `1024`。显式填写越界值会导致配置加载失败。

## 构建与测试

在仓库根目录使用 Go 1.26、Python 3.10 或更新版本，并把 [Zig 官方发行包](https://ziglang.org/download/)中的 `zig` 加到 PATH。CI 使用 Zig 0.15.2。版本记录在 [VERSION](VERSION)，可执行文件输出到 `release/mosdns-xiaomi-be7000-<版本>`。

```sh
go test ./...
python release.py
```

## 发布

GitHub Actions 验证主分支的构建、静态链接、程序版本和二进制规则加载后，为 [VERSION](VERSION) 中的新版本发布可执行文件和 `SHA256SUMS.txt`。发布标签与可执行文件同名，例如 `mosdns-xiaomi-be7000-0.0.4`。

## 许可证

[GNU GPL v3 或更高版本](LICENSE)。静态链接构建使用 Zig 0.15.2 提供的工具链和 musl，相关声明见 [Zig 许可证](LICENSES/zig.txt) 和 [musl 版权声明](LICENSES/musl.txt)。
