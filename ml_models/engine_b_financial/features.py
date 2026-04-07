"""
Extract quarterly financial features from stock-analyzer JSON data.
"""
import json
import os
import numpy as np


def safe_get(mapping, year, key, default=0.0):
    if mapping is None:
        return default
    yd = mapping.get(year, {})
    return yd.get(key, default) if isinstance(yd, dict) else default


def load_json(path):
    if not os.path.exists(path):
        return None
    with open(path, 'r', encoding='utf-8') as f:
        return json.load(f)


def extract_features_from_symbol(stock_dir):
    """
    stock_dir: e.g. ~/.config/stock-analyzer/data/000001.SZ
    Returns: list of dicts ordered by year desc (latest first)
    """
    bs = load_json(os.path.join(stock_dir, "balance_sheet.json"))
    inc = load_json(os.path.join(stock_dir, "income_statement.json"))
    cf = load_json(os.path.join(stock_dir, "cash_flow.json"))
    if bs is None or inc is None or cf is None:
        return []

    # Determine common years
    years = sorted(
        set(bs.keys()) & set(inc.keys()) & set(cf.keys()),
        reverse=True
    )
    records = []
    for y in years:
        revenue = safe_get(inc, y, '营业收入', 0.0)
        cost = safe_get(inc, y, '营业成本', 0.0)
        net_profit = safe_get(inc, y, '净利润', 0.0)
        total_assets = safe_get(bs, y, '总资产', 0.0)
        total_liabilities = safe_get(bs, y, '总负债', 0.0)
        equity = safe_get(bs, y, '所有者权益合计', total_assets - total_liabilities)
        if equity <= 0:
            equity = total_assets - total_liabilities
        if equity <= 0:
            equity = 1e-6

        gross_profit = revenue - cost
        gross_margin = gross_profit / revenue if revenue > 0 else 0.0

        roe = net_profit / equity

        debt_ratio = total_liabilities / total_assets if total_assets > 0 else 0.0

        op_cash = safe_get(cf, y, '经营活动产生的现金流量净额', 0.0)
        cash_ratio = op_cash / net_profit if net_profit != 0 else 0.0

        inventory = safe_get(bs, y, '存货', 0.0)
        receivables = safe_get(bs, y, '应收账款', 0.0)
        turnover = revenue / (inventory + receivables) if (inventory + receivables) > 0 else 0.0

        # Simplified M-Score proxy using accruals / total assets
        accruals = net_profit - op_cash
        mscore_proxy = -accruals / total_assets if total_assets > 0 else 0.0

        records.append({
            'year': y,
            'roe': roe,
            'gross_margin': gross_margin,
            'debt_ratio': debt_ratio,
            'cash_ratio': cash_ratio,
            'turnover': turnover,
            'mscore_proxy': mscore_proxy,
            'revenue': revenue,
            'net_profit': net_profit,
        })
    return records


def build_dataset(data_dir, seq_len=8):
    """
    Scan data_dir for all symbols and build (X, y) tensors.
    """
    X, y_roe, y_rev, y_mscore, y_health = [], [], [], [], []

    if not os.path.exists(data_dir):
        return None, None, None, None, None

    for symbol in os.listdir(data_dir):
        stock_dir = os.path.join(data_dir, symbol)
        if not os.path.isdir(stock_dir):
            continue
        recs = extract_features_from_symbol(stock_dir)
        if len(recs) < seq_len + 1:
            continue

        for i in range(len(recs) - seq_len):
            window = recs[i:i + seq_len]  # latest first
            next_q = recs[i + seq_len]

            feat = np.array([
                [r['roe'], r['gross_margin'], r['debt_ratio'],
                 r['cash_ratio'], r['turnover'], r['mscore_proxy'],
                 r['revenue'] / 1e8, r['net_profit'] / 1e8]
                for r in window
            ], dtype=np.float32)

            # direction labels
            def dir_label(curr, nxt, thr=0.005):
                delta = (nxt - curr) / (abs(curr) + 1e-6)
                if delta > thr:
                    return 2  # up
                elif delta < -thr:
                    return 0  # down
                return 1  # flat

            y_roe.append(dir_label(window[0]['roe'], next_q['roe']))
            y_rev.append(dir_label(window[0]['revenue'], next_q['revenue']))
            y_mscore.append(dir_label(window[0]['mscore_proxy'], next_q['mscore_proxy']))

            # health score synthetic: weighted sum normalized to 0-100
            h = 50 \
                + 20 * (window[0]['roe'] > 0.1) \
                + 15 * (window[0]['gross_margin'] > 0.3) \
                - 15 * (window[0]['debt_ratio'] > 0.6) \
                + 10 * (window[0]['cash_ratio'] > 1.0) \
                - 10 * (window[0]['mscore_proxy'] < -0.05)
            y_health.append(np.clip(h, 0, 100))
            X.append(feat)

    if len(X) == 0:
        return None, None, None, None, None

    return (
        np.stack(X),
        np.array(y_roe, dtype=np.int64),
        np.array(y_rev, dtype=np.int64),
        np.array(y_mscore, dtype=np.int64),
        np.array(y_health, dtype=np.float32),
    )


def generate_synthetic_data(n_samples=2000, seq_len=8):
    np.random.seed(42)
    X = []
    y_roe, y_rev, y_mscore, y_health = [], [], [], []
    for _ in range(n_samples):
        roe = np.random.randn(seq_len).cumsum() * 0.02 + 0.08
        gm = np.random.randn(seq_len).cumsum() * 0.03 + 0.35
        debt = np.random.randn(seq_len).cumsum() * 0.05 + 0.45
        cash = np.random.randn(seq_len).cumsum() * 0.2 + 1.1
        turn = np.random.randn(seq_len).cumsum() * 0.5 + 2.0
        mscore = np.random.randn(seq_len).cumsum() * 0.05 - 0.1
        rev = np.exp(np.random.randn(seq_len).cumsum() * 0.1)
        profit = rev * roe * np.random.uniform(0.8, 1.2, seq_len)

        feat = np.stack([roe, gm, debt, cash, turn, mscore, rev, profit], axis=1).astype(np.float32)
        X.append(feat)

        # labels from next step simulation
        def dir_label(v):
            d = v[1] - v[0] if len(v) > 1 else 0
            thr = abs(v[0]) * 0.05 + 0.005
            if d > thr:
                return 2
            if d < -thr:
                return 0
            return 1

        y_roe.append(dir_label(roe))
        y_rev.append(dir_label(rev))
        y_mscore.append(dir_label(mscore))

        h = 50 + 20 * (roe[0] > 0.1) + 15 * (gm[0] > 0.3) - 15 * (debt[0] > 0.6) + 10 * (cash[0] > 1.0) - 10 * (mscore[0] < -0.05)
        y_health.append(np.clip(h, 0, 100))

    return (
        np.stack(X),
        np.array(y_roe, dtype=np.int64),
        np.array(y_rev, dtype=np.int64),
        np.array(y_mscore, dtype=np.int64),
        np.array(y_health, dtype=np.float32),
    )
