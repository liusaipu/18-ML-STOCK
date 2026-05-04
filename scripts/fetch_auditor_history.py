#!/usr/bin/env python3
"""
获取股票历年审计机构信息
数据来源：巨潮资讯网(cninfo.com.cn) 公告查询
输入：{"symbol": "000001.SZ"}
输出：{
  "auditor_name": "xxx",
  "auditor_changed": true/false,
  "history": [...],
  "change_details": [
    {
      "date": "2025-12-11",
      "old_auditor": "永拓",
      "new_auditor": "",
      "reason": "拟变更会计师事务所",
      "is_before_annual_report": true,
      "annual_report_deadline": "2026-04-30",
      "raw_title": "关于拟变更会计师事务所的公告"
    }
  ]
}
"""
import json
import sys
import os
import re
from datetime import datetime, timedelta

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, SCRIPT_DIR)
from cninfo_utils import query_announcements, extract_year_from_title


# 常见会计师事务所名称（用于从标题中提取，按优先级排序：完整名称优先）
KNOWN_AUDITORS = [
    # 完整名称（含组织形式）
    "普华永道中天会计师事务所（特殊普通合伙）",
    "安永华明会计师事务所（特殊普通合伙）",
    "毕马威华振会计师事务所（特殊普通合伙）",
    "德勤华永会计师事务所（特殊普通合伙）",
    "天健会计师事务所（特殊普通合伙）",
    "立信会计师事务所（特殊普通合伙）",
    "大华会计师事务所（特殊普通合伙）",
    "容诚会计师事务所（特殊普通合伙）",
    "天职国际会计师事务所（特殊普通合伙）",
    "信永中和会计师事务所（特殊普通合伙）",
    "中兴华会计师事务所（特殊普通合伙）",
    "中审众环会计师事务所（特殊普通合伙）",
    "大信会计师事务所（特殊普通合伙）",
    "瑞华会计师事务所（特殊普通合伙）",
    "致同会计师事务所（特殊普通合伙）",
    "中汇会计师事务所（特殊普通合伙）",
    "中喜会计师事务所（特殊普通合伙）",
    "中审亚太会计师事务所（特殊普通合伙）",
    "公证天业会计师事务所（特殊普通合伙）",
    "华兴会计师事务所（特殊普通合伙）",
    "永拓会计师事务所（特殊普通合伙）",
    "亚太（集团）会计师事务所（特殊普通合伙）",
    "希格玛会计师事务所（特殊普通合伙）",
    "中准会计师事务所（特殊普通合伙）",
    "上会会计师事务所（特殊普通合伙）",
    "众华会计师事务所（特殊普通合伙）",
    "苏亚金诚会计师事务所（特殊普通合伙）",
    "利安达会计师事务所（特殊普通合伙）",
    # 简称
    "普华永道", "安永", "毕马威", "德勤",
    "天健", "立信", "大华", "容诚", "天职国际", "信永中和",
    "中兴华", "中审众环", "大信", "瑞华", "致同",
    "中汇", "中喜", "中审亚太", "公证天业", "华兴",
    "永拓", "亚太", "希格玛", "中准", "上会", "众华",
    "苏亚金诚", "利安达",
]


def extract_auditor_from_title(title: str) -> str:
    """从标题中提取审计机构名称（返回最完整的匹配）"""
    best_match = ""
    for name in KNOWN_AUDITORS:
        if name in title:
            if len(name) > len(best_match):
                best_match = name
    return best_match


def is_change_announcement(title: str) -> bool:
    """判断是否为审计机构变更类公告"""
    return any(kw in title for kw in ["变更", "更换", "改聘", "解聘", "不再续聘", "终止合作"])


# 政策合规更换关键词（国企8年强制轮换等）
POLICY_COMPLIANCE_KEYWORDS = [
    "轮换期届满", "聘任期限届满", "服务年限届满", "强制轮换",
    "聘期届满", "合同期限届满", "审计年限届满", "连续服务年限",
    "达到轮换年限", "轮换期限", "服务期满",
]

# 异常更换关键词（需警惕）
ABNORMAL_KEYWORDS = [
    "无法达成一致", "审计范围受限", "审计意见分歧", "独立性",
    "辞任", "辞聘", "主动辞任", "被解聘",
]


def infer_change_reason(title: str) -> str:
    """从标题推断变更原因"""
    if "不再续聘" in title or "终止合作" in title:
        return "不再续聘原审计机构"
    if "解聘" in title:
        return "解聘原审计机构"
    if "改聘" in title:
        return "改聘审计机构"
    if "拟变更" in title or "拟改聘" in title:
        return "拟变更审计机构"
    if "变更" in title:
        return "变更审计机构"
    if "更换" in title:
        return "更换审计机构"
    return "审计机构变更"


def is_policy_compliance_change(title: str) -> bool:
    """判断是否为政策合规更换（如国企8年强制轮换）"""
    return any(kw in title for kw in POLICY_COMPLIANCE_KEYWORDS)


def is_abnormal_change(title: str) -> bool:
    """判断是否为异常更换（需警惕）"""
    return any(kw in title for kw in ABNORMAL_KEYWORDS)


def is_before_annual_report(date_str: str) -> tuple:
    """判断变更日期是否在年报披露期间（1月1日-4月30日）
    返回: (是否年报前, 对应年报截止日)"""
    try:
        dt = datetime.strptime(date_str, "%Y-%m-%d")
        month = dt.month
        if 1 <= month <= 4:
            # 年报截止日为同年4月30日
            return True, f"{dt.year}-04-30"
        else:
            # 变更日期在5月及之后，对应次年年报
            return False, f"{dt.year + 1}-04-30"
    except Exception:
        return False, ""


def build_change_details(announcements: list) -> list:
    """从公告列表中构建结构化变更详情"""
    change_details = []
    
    # 按日期排序
    sorted_anns = sorted(announcements, key=lambda x: x.get("announcementDate", ""))
    
    for i, a in enumerate(sorted_anns):
        title = a.get("announcementTitle", "")
        date = a.get("announcementDate", "")
        
        if not is_change_announcement(title):
            continue
        
        # 推断旧事务所：查找此前最近的一个含审计机构名称的公告
        old_auditor = ""
        for j in range(i - 1, -1, -1):
            prev_title = sorted_anns[j].get("announcementTitle", "")
            prev_auditor = extract_auditor_from_title(prev_title)
            if prev_auditor:
                old_auditor = prev_auditor
                break
        
        # 推断新事务所：查找此后最近的一个含审计机构名称的公告
        new_auditor = ""
        for j in range(i + 1, len(sorted_anns)):
            next_title = sorted_anns[j].get("announcementTitle", "")
            next_auditor = extract_auditor_from_title(next_title)
            if next_auditor and next_auditor != old_auditor:
                new_auditor = next_auditor
                break
        
        is_before, deadline = is_before_annual_report(date)
        
        change_details.append({
            "date": date,
            "old_auditor": old_auditor,
            "new_auditor": new_auditor,
            "reason": infer_change_reason(title),
            "is_before_annual_report": is_before,
            "annual_report_deadline": deadline,
            "raw_title": title,
            "is_policy_compliance": is_policy_compliance_change(title),
            "is_abnormal": is_abnormal_change(title),
        })
    
    return change_details


def fetch_auditor_history(symbol: str):
    """获取股票历年审计机构信息"""
    try:
        # 查询近3年的会计师事务所相关公告
        start = (datetime.now() - timedelta(days=365 * 3)).strftime("%Y-%m-%d")
        announcements = query_announcements(
            symbol,
            search_key="会计师事务所",
            start_date=start,
            page_size=50,
        )

        if announcements and "error" in announcements[0]:
            return {
                "auditor_name": "",
                "auditor_changed": False,
                "history": [],
                "change_details": [],
                "error": announcements[0]["error"],
            }

        # 按日期排序（用于历史推断）
        sorted_anns = sorted(announcements, key=lambda x: x.get("announcementDate", ""))
        
        history_records = []
        has_change = False
        current_auditor = ""
        
        # 查找当前审计机构（最新一条含审计机构名称的公告中的机构）
        for a in reversed(sorted_anns):
            title = a.get("announcementTitle", "")
            # 跳过变更公告本身
            if is_change_announcement(title):
                continue
            auditor = extract_auditor_from_title(title)
            if auditor:
                current_auditor = auditor
                break
        
        for a in sorted_anns:
            title = a.get("announcementTitle", "")
            date = a.get("announcementDate", "")
            is_change = is_change_announcement(title)
            if is_change:
                has_change = True
            # 尝试提取审计机构名称
            auditor = extract_auditor_from_title(title)
            year = extract_year_from_title(title) or date[:4]
            change_flag = " [变更]" if is_change else ""
            auditor_str = f" ({auditor})" if auditor else ""
            history_records.append(f"[{year}] [{date}]{change_flag}{auditor_str} {title}")

        # 构建结构化变更详情
        change_details = build_change_details(sorted_anns)

        return {
            "auditor_name": current_auditor,
            "auditor_changed": has_change,
            "history": history_records[:10],
            "change_details": change_details,
        }

    except Exception as e:
        return {
            "auditor_name": "",
            "auditor_changed": False,
            "history": [],
            "change_details": [],
            "error": str(e),
        }


def main():
    req = json.load(sys.stdin)
    symbol = req.get("symbol", "")
    result = fetch_auditor_history(symbol)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
