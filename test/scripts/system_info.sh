#!/bin/bash
# 系统信息收集脚本
# 收集并显示系统基本信息

# @param output_format 输出格式（text/json） 
# @param show_disk 是否显示磁盘信息（yes/no）
# @param show_network 是否显示网络信息（yes/no）

# 初始化变量
OUTPUT_FORMAT="text"
SHOW_DISK="yes"
SHOW_NETWORK="yes"

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --output_format)
            OUTPUT_FORMAT="$2"
            shift 2
            ;;
        --show_disk)
            SHOW_DISK="$2"
            shift 2
            ;;
        --show_network)
            SHOW_NETWORK="$2"
            shift 2
            ;;
        *)
            echo "未知参数: $1"
            exit 1
            ;;
    esac
done

echo "=== 系统信息收集 ==="
echo "时间: $(date)"
echo "主机名: $(hostname)"
echo "系统: $(uname -a)"
echo ""

# CPU信息
echo "=== CPU信息 ==="
echo "CPU型号: $(grep "model name" /proc/cpuinfo | head -1 | cut -d: -f2 | xargs)"
echo "CPU核心数: $(nproc)"
echo "CPU使用率: $(top -bn1 | grep "Cpu(s)" | awk '{print $2}')"
echo ""

# 内存信息
echo "=== 内存信息 ==="
free -h
echo ""

# 磁盘信息
if [ "$SHOW_DISK" == "yes" ]; then
    echo "=== 磁盘使用情况 ==="
    df -h
    echo ""
fi

# 网络信息
if [ "$SHOW_NETWORK" == "yes" ]; then
    echo "=== 网络接口 ==="
    ip -br addr show
    echo ""
fi

echo "=== 收集完成 ==="