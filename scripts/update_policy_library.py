#!/usr/bin/env python3
"""
动态更新十五五政策库
用法: python3 update_policy_library.py <data_dir>
"""

import json
import os
import re
import sys
from datetime import datetime

import akshare as ak


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
