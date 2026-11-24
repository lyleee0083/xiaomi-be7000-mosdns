# 小米 BE7000 MosDNS

基于 [MosDNS v5](https://github.com/IrineSistiana/mosdns) 的 DNS 转发器，适用于小米 BE7000。

## 功能

- 为 `qualcommax/ipq95xx` 固件构建 Linux ARM64 可执行文件，使用 ARMv8.0 指令基线及静态链接。
- 启用延迟缓存时，同一缓存键已有后台刷新任务的请求会直接跳过重复刷新和查询上下文复制。

## 快速开始

从 [GitHub Actions](https://github.com/lyleee0083/xiaomi-be7000-mosdns/actions) 或 [Releases](https://github.com/lyleee0083/xiaomi-be7000-mosdns/releases) 下载 `mosdns-xiaomi-be7000-linux-arm64.zip`。解压后，在目标 Linux ARM64 环境验证二进制版本。

```sh
./mosdns version
```

## 配置

归档包含二进制、说明和许可证。MosDNS 继续读取启动参数指定的配置文件。插件、上游、监听地址和运行目录沿用原有配置方式，详见 [MosDNS Wiki](https://irine-sistiana.gitbook.io/mosdns-wiki/)。

## 构建与测试

在仓库根目录使用 Git、Go 1.26 和 Python 3.10 或更新版本。构建产物位于 `release/mosdns-xiaomi-be7000-linux-arm64.zip`。

```sh
go test ./...
python release.py
```

## 发布

测试工作流提供 BE7000 构建产物。推送 `v*` 标签时，发布工作流将 BE7000 归档上传到该标签对应的 GitHub Release。

## 许可证

[GNU GPL v3 或更高版本](LICENSE)。
