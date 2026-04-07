"""
Engine A: Sentiment + Price High-Frequency Abnormal-Movement Warning
Input:
  - text_seq: [batch, seq_len, text_dim]
  - price_seq: [batch, seq_len, price_dim]
Output:
  - movement_logits: [batch, 3]  (0=down, 1=flat, 2=up)
  - prob: [batch, 1]  abnormal probability [0~1]
"""
import math
import torch
import torch.nn as nn


class PositionalEncoding(nn.Module):
    def __init__(self, d_model, max_len=64):
        super().__init__()
        pe = torch.zeros(max_len, d_model)
        position = torch.arange(0, max_len, dtype=torch.float).unsqueeze(1)
        div_term = torch.exp(torch.arange(0, d_model, 2).float() * (-math.log(10000.0) / d_model))
        pe[:, 0::2] = torch.sin(position * div_term)
        if d_model % 2 == 1:
            pe[:, 1::2] = torch.cos(position * div_term[:-1])
        else:
            pe[:, 1::2] = torch.cos(position * div_term)
        self.register_buffer('pe', pe)

    def forward(self, x):
        return x + self.pe[:x.size(1), :]


class CrossAttention(nn.Module):
    def __init__(self, q_dim, kv_dim, hidden_dim):
        super().__init__()
        self.Wq = nn.Linear(q_dim, hidden_dim)
        self.Wk = nn.Linear(kv_dim, hidden_dim)
        self.Wv = nn.Linear(kv_dim, hidden_dim)
        self.scale = math.sqrt(hidden_dim)

    def forward(self, query, kv):
        # query: [B, T, q_dim], kv: [B, T, kv_dim]
        Q = self.Wq(query)
        K = self.Wk(kv)
        V = self.Wv(kv)
        scores = torch.matmul(Q, K.transpose(-2, -1)) / self.scale
        attn = torch.softmax(scores, dim=-1)
        out = torch.matmul(attn, V)
        return out, attn


class SentimentPriceFusion(nn.Module):
    def __init__(self, text_dim=32, price_dim=24, hidden_dim=48, nhead=4, num_layers=2, dropout=0.2):
        super().__init__()
        self.text_pe = PositionalEncoding(text_dim)
        encoder_layer_t = nn.TransformerEncoderLayer(d_model=text_dim, nhead=nhead,
                                                     dim_feedforward=text_dim * 2,
                                                     dropout=dropout, batch_first=True)
        self.text_encoder = nn.TransformerEncoder(encoder_layer_t, num_layers=num_layers)

        self.price_pe = PositionalEncoding(price_dim)
        encoder_layer_p = nn.TransformerEncoderLayer(d_model=price_dim, nhead=nhead,
                                                     dim_feedforward=price_dim * 2,
                                                     dropout=dropout, batch_first=True)
        self.price_encoder = nn.TransformerEncoder(encoder_layer_p, num_layers=num_layers)

        self.cross_attn = CrossAttention(text_dim, price_dim, hidden_dim)

        fusion_dim = hidden_dim + price_dim
        self.mlp = nn.Sequential(
            nn.Linear(fusion_dim, fusion_dim),
            nn.ReLU(),
            nn.Dropout(dropout),
            nn.Linear(fusion_dim, 64),
            nn.ReLU(),
            nn.Dropout(dropout),
        )
        self.head_movement = nn.Linear(64, 3)
        self.head_prob = nn.Linear(64, 1)

    def forward(self, text_seq, price_seq):
        # text_seq: [B, T, text_dim]
        # price_seq: [B, T, price_dim]
        t = self.text_pe(text_seq)
        t = self.text_encoder(t)  # [B, T, text_dim]

        p = self.price_pe(price_seq)
        p = self.price_encoder(p)  # [B, T, price_dim]

        fused, cross_weights = self.cross_attn(t, p)  # [B, T, hidden_dim]

        # mean pooling over time
        fused_pool = fused.mean(dim=1)  # [B, hidden_dim]
        price_pool = p.mean(dim=1)      # [B, price_dim]

        concat = torch.cat([fused_pool, price_pool], dim=-1)
        features = self.mlp(concat)
        movement_logits = self.head_movement(features)
        prob = torch.sigmoid(self.head_prob(features))
        return {
            'movement': movement_logits,
            'prob': prob,
            'cross_weights': cross_weights,
        }


def build_model(text_dim=32, price_dim=24):
    return SentimentPriceFusion(text_dim=text_dim, price_dim=price_dim)
