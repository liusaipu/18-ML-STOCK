#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
动态更新十五五政策库
用法: python3 update_policy_library.py <data_dir>
"""

import json
import os
import re
import sys
import io
from datetime import datetime

# 强制 UTF-8 输出，避免 Windows GBK 乱码
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")
    sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding="utf-8")
    os.environ["PYTHONIOENCODING"] = "utf-8"

import akshare as ak

# 禁用 akshare 内部所有 tqdm 进度条，防止进度条污染 stdout 导致 Go 端 JSON 解析失败
from akshare.utils import tqdm as _tqdm_module
_tqdm_module.get_tqdm = lambda enable=True: lambda iterable, *args, **kwargs: iterable


def extract_policy_keywords(text: str) -> list:
    """从文本中提取潜在的政策关键词（轻量级正则）"""
    text = text.strip()
    if not text or len(text) < 4:
        return []

    # 预定义的高频政策方向词库
    seed_keywords = {
        "国产替代", "AI算力", "数字经济", "先进制造", "智能制造",
        "信创", "边缘计算", "人工智能", "数据要素", "平台经济",
        "5G", "6G", "算力网络", "卫星通信", "消费电子",
        "MR混合现实", "消费升级", "出口贸易", "双碳目标",
        "新能源", "光伏", "风电", "储能", "固态电池",
        "绿色转型", "电力市场化", "绿电", "虚拟电厂", "能源安全",
        "特高压", "智能电网", "新能源车", "智能驾驶", "出海战略",
        "以旧换新", "健康中国", "创新药", "银发经济", "生物医药",
        "中药传承", "疫苗", "AI医疗", "高端医疗装备", "民营医院",
        "CXO", "医药流通", "连锁药房", "国防现代化", "低空经济",
        "商业航天", "大飞机", "海工装备", "高端装备", "一带一路",
        "工业母机", "机器人", "新型城镇化", "基建", "水利工程",
        "粮食安全", "乡村振兴", "农业现代化", "大消费", "预制菜",
        "智能家居", "旅游消费", "冰雪经济", "文化强国", "数字文化",
        "短剧游戏", "IP经济", "元宇宙", "纺织服饰", "跨境电商",
        "新零售", "免税概念", "医美", "化妆品", "金融强国",
        "数字金融", "高股息", "中特估", "财富管理", "资本市场改革",
        "养老金融", "地产稳预期", "保障房", "绿色建材", "装配式建筑",
        "新能源金属", "稀土永磁", "战略性矿产", "黄金储备", "避险资产",
        "煤化工", "绿色矿山", "油气开采", "国企改革", "新材料",
        "可降解塑料", "绿色建筑", "碳纤维", "污水处理", "固废处理",
        "城市燃气", "海绵城市", "清洁供暖", "物流强国", "快递",
        "冷链物流", "港口", "交通强国", "出境游", "消费复苏",
        "教育强国", "职业教育", "AI教育", "黄金珠宝",
    }

    found = []
    for kw in seed_keywords:
        if kw in text:
            found.append(kw)
    return found


def merge_keywords(existing: list, new_items: list) -> list:
    """合并关键词列表，去重，保持原有顺序并追加新项"""
    seen = set(existing)
    merged = list(existing)
    for item in new_items:
        if item not in seen:
            seen.add(item)
            merged.append(item)
    return merged


# 内置默认行业政策映射（与 Go 端 defaultIndustryPolicyMap 保持一致）
DEFAULT_INDUSTRY_POLICIES = {
    "半导体": ["国产替代", "AI算力", "数字经济", "先进制造"],
    "电子元件": ["国产替代", "消费电子", "AI硬件", "智能制造"],
    "计算机设备": ["信创", "AI算力", "数字经济", "边缘计算"],
    "软件开发": ["人工智能", "信创", "数字经济", "数据要素"],
    "互联网服务": ["平台经济", "数字经济", "人工智能"],
    "通信设备": ["5G/6G", "算力网络", "卫星通信", "数字经济"],
    "光学光电子": ["消费电子", "MR混合现实", "智能制造"],
    "消费电子": ["消费升级", "智能制造", "出口贸易"],
    "新能源": ["双碳目标", "新能源", "储能", "绿色转型"],
    "光伏设备": ["双碳目标", "光伏", "储能", "新能源"],
    "风电设备": ["双碳目标", "风电", "储能", "新能源"],
    "电池": ["新能源车", "储能", "固态电池", "绿色转型"],
    "电力行业": ["电力市场化", "绿电", "虚拟电厂", "能源安全"],
    "电网设备": ["特高压", "智能电网", "储能", "能源安全"],
    "电机": ["新能源车", "智能制造", "风电"],
    "汽车整车": ["新能源车", "智能制造", "出海战略", "以旧换新"],
    "汽车零部件": ["新能源车", "智能驾驶", "智能制造", "出海战略"],
    "医药制造": ["健康中国", "创新药", "银发经济", "生物医药"],
    "化学制药": ["健康中国", "创新药", "原料药", "生物医药"],
    "中药": ["健康中国", "中药传承", "银发经济"],
    "生物制品": ["健康中国", "创新药", "疫苗", "生物医药"],
    "医疗器械": ["健康中国", "AI医疗", "银发经济", "高端医疗装备"],
    "医疗服务": ["健康中国", "民营医院", "银发经济", "CXO"],
    "医药商业": ["健康中国", "医药流通", "连锁药房"],
    "航天航空": ["国防现代化", "低空经济", "商业航天", "大飞机"],
    "船舶制造": ["国防现代化", "海工装备", "高端装备", "一带一路"],
    "专用设备": ["智能制造", "工业母机", "机器人", "高端装备"],
    "通用设备": ["智能制造", "工业母机", "机器人", "高端装备"],
    "工程机械": ["一带一路", "新型城镇化", "高端装备", "智能制造"],
    "农牧饲渔": ["粮食安全", "乡村振兴", "农业现代化"],
    "食品饮料": ["扩内需", "大消费", "乡村振兴", "预制菜"],
    "酿酒行业": ["扩内需", "大消费", "消费升级"],
    "家电行业": ["扩内需", "以旧换新", "智能家居", "出海战略"],
    "旅游酒店": ["扩内需", "旅游消费", "冰雪经济", "消费复苏"],
    "文化传媒": ["文化强国", "数字文化", "短剧游戏", "IP经济"],
    "游戏": ["数字经济", "文化强国", "短剧游戏", "IP经济"],
    "纺织服装": ["纺织服饰", "跨境电商", "新零售", "出口贸易"],
    "商业百货": ["新零售", "免税概念", "扩内需", "大消费"],
    "美容护理": ["消费升级", "医美", "化妆品", "健康中国"],
    "保险": ["金融强国", "数字金融", "养老金融", "财富管理"],
    "银行": ["金融强国", "高股息", "中特估", "数字金融"],
    "证券": ["金融强国", "资本市场改革", "财富管理"],
    "房地产开发": ["地产稳预期", "保障房", "新型城镇化"],
    "装修建材": ["绿色建材", "装配式建筑", "保障房", "以旧换新"],
    "装修装饰": ["绿色建筑", "装配式建筑", "保障房", "以旧换新"],
    "工程咨询服务": ["基建", "新型城镇化", "水利工程"],
    "工程建设": ["基建", "新型城镇化", "水利工程", "一带一路"],
    "水泥建材": ["基建", "绿色建材", "装配式建筑"],
    "钢铁行业": ["新能源金属", "战略性矿产", "高端装备", "绿色矿山"],
    "有色金属": ["新能源金属", "稀土永磁", "战略性矿产", "黄金储备"],
    "贵金属": ["黄金储备", "避险资产", "战略性矿产"],
    "煤炭行业": ["能源安全", "煤化工", "绿色矿山"],
    "石油行业": ["能源安全", "油气开采"],
    "采掘行业": ["能源安全", "战略性矿产", "绿色矿山"],
    "燃气": ["城市燃气", "清洁供暖", "能源安全"],
    "公用事业": ["绿电", "虚拟电厂", "海绵城市", "清洁供暖"],
    "环保行业": ["双碳目标", "污水处理", "固废处理", "绿色转型"],
    "物流行业": ["物流强国", "快递", "冷链物流", "交通强国"],
    "航运港口": ["交通强国", "港口", "一带一路", "出海战略"],
    "铁路公路": ["交通强国", "基建", "一带一路"],
    "航空机场": ["交通强国", "出境游", "消费复苏", "低空经济"],
    "教育": ["教育强国", "职业教育", "AI教育"],
    "珠宝首饰": ["消费升级", "黄金珠宝"],
    "包装材料": ["可降解塑料", "绿色包装", "智能制造"],
    "造纸印刷": ["文化强国", "数字文化", "绿色转型"],
    "化学制品": ["新材料", "可降解塑料", "绿色建材", "碳纤维"],
    "橡胶制品": ["新能源车", "智能制造", "新材料"],
    "塑料制品": ["新材料", "可降解塑料", "绿色建材"],
    "玻璃玻纤": ["新能源", "光伏", "绿色建筑", "智能制造"],
    "非金属材料": ["新材料", "绿色建材", "战略性矿产"],
    "仪器仪表": ["智能制造", "高端装备", "科学仪器"],
    "金属制品": ["高端装备", "智能制造", "新材料"],
    "交运设备": ["高端装备", "智能制造", "交通强国"],
    "综合行业": ["数字经济", "智能制造", "大消费"],
}

DEFAULT_CONCEPT_POLICIES = {
    "半导体": ["国产替代", "AI算力"],
    "芯片": ["国产替代", "AI算力"],
    "光刻机": ["国产替代", "先进制造"],
    "人工智能": ["人工智能", "数字经济"],
    "算力": ["AI算力", "数字经济"],
    "信创": ["信创", "数字经济"],
    "数据中心": ["数字经济", "算力网络"],
    "云计算": ["数字经济", "人工智能"],
    "5G": ["5G/6G", "数字经济"],
    "6G": ["5G/6G", "卫星通信"],
    "卫星导航": ["卫星通信", "国防现代化"],
    "消费电子": ["消费电子", "智能制造"],
    "MR头显": ["MR混合现实", "消费电子"],
    "虚拟现实": ["MR混合现实", "元宇宙"],
    "元宇宙": ["元宇宙", "数字文化"],
    "新能源": ["双碳目标", "新能源"],
    "光伏": ["双碳目标", "光伏"],
    "储能": ["储能", "绿色转型"],
    "锂电池": ["新能源车", "储能"],
    "固态电池": ["新能源车", "储能"],
    "机器人": ["智能制造", "机器人"],
    "无人机": ["低空经济", "国防现代化"],
    "低空经济": ["低空经济", "智能制造"],
    "商业航天": ["商业航天", "国防现代化"],
    "创新药": ["健康中国", "创新药"],
    "中药": ["健康中国", "中药传承"],
    "白酒": ["扩内需", "大消费"],
    "军工": ["国防现代化", "高端装备"],
    "一带一路": ["一带一路", "出口贸易"],
    "出海": ["出海战略", "出口贸易"],
    "高股息": ["高股息", "中特估"],
    "中特估": ["中特估", "国企改革"],
    "国企改革": ["国企改革", "中特估"],
    "数字经济": ["数字经济", "人工智能"],
    "数据要素": ["数字经济", "数据要素"],
    "信创": ["信创", "数字经济"],
    "银发经济": ["银发经济", "健康中国"],
    "医美": ["消费升级", "健康中国"],
    "冰雪经济": ["扩内需", "旅游消费"],
    "以旧换新": ["扩内需", "以旧换新"],
}


def init_default_library(lib: dict) -> dict:
    """如果库中 industries/concepts 为空，填充默认值"""
    if not lib.get("industries"):
        lib["industries"] = {}
        for k, v in DEFAULT_INDUSTRY_POLICIES.items():
            lib["industries"][k] = list(v)
    if not lib.get("concepts"):
        lib["concepts"] = {}
        for k, v in DEFAULT_CONCEPT_POLICIES.items():
            lib["concepts"][k] = list(v)
    return lib


def main():
    if len(sys.argv) < 2:
        print("Usage: update_policy_library.py <data_dir>", file=sys.stderr)
        sys.exit(1)

    data_dir = sys.argv[1]
    policy_path = os.path.join(data_dir, "policy_library.json")

    # 加载现有库
    if os.path.exists(policy_path):
        with open(policy_path, "r", encoding="utf-8") as f:
            lib = json.load(f)
    else:
        lib = {"version": "1.0", "updated_at": "", "industries": {}, "concepts": {}}

    # 确保字段存在
    lib.setdefault("industries", {})
    lib.setdefault("concepts", {})

    # 如果为空，先填充默认值
    lib = init_default_library(lib)

    added_industry_keywords = 0
    added_concept_keywords = 0
    errors = []

    # 1. 从 CCTV 新闻提取政策关键词 -> 作为通用概念补充
    try:
        df_news = ak.news_cctv(date=datetime.now().strftime("%Y%m%d"))
        if df_news is not None and not df_news.empty:
            for _, row in df_news.head(20).iterrows():
                title = str(row.get("title", ""))
                kws = extract_policy_keywords(title)
                if kws:
                    old = lib["concepts"].get("政策热点", [])
                    merged = merge_keywords(old, kws)
                    if len(merged) > len(old):
                        added_concept_keywords += len(merged) - len(old)
                    lib["concepts"]["政策热点"] = merged
    except Exception as e:
        errors.append(f"news_cctv: {e}")

    # 2. 从东方财富概念板块提取热点概念 -> 映射为政策概念
    try:
        df_board = ak.stock_board_concept_name_em()
        if df_board is not None and not df_board.empty:
            # 取涨幅靠前/热度高的板块名称
            hot_boards = df_board.head(50)
            for _, row in hot_boards.iterrows():
                name = str(row.get("板块名称", ""))
                if not name:
                    continue
                kws = extract_policy_keywords(name)
                if kws:
                    old = lib["concepts"].get(name, [])
                    merged = merge_keywords(old, kws)
                    if len(merged) > len(old):
                        added_concept_keywords += len(merged) - len(old)
                    lib["concepts"][name] = merged
    except Exception as e:
        errors.append(f"stock_board_concept_name_em: {e}")

    # 3. 尝试把新概念同步到 industries（如果行业名与概念名匹配）
    for industry, policies in list(lib["industries"].items()):
        if industry in lib["concepts"]:
            new_policies = lib["concepts"][industry]
            merged = merge_keywords(policies, new_policies)
            if len(merged) > len(policies):
                added_industry_keywords += len(merged) - len(policies)
            lib["industries"][industry] = merged

    # 更新元信息
    lib["version"] = "auto"
    lib["updated_at"] = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")

    # 写回文件
    with open(policy_path, "w", encoding="utf-8") as f:
        json.dump(lib, f, ensure_ascii=False, indent=2)

    result = {
        "success": True,
        "path": policy_path,
        "added_industry_keywords": added_industry_keywords,
        "added_concept_keywords": added_concept_keywords,
        "total_industries": len(lib["industries"]),
        "total_concepts": len(lib["concepts"]),
        "errors": errors,
    }
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
