"""
Engine B: Financial Trend Warning (BiLSTM + Self-Attention + MultiTask)
Input: 8 quarters x N_features
Output:
  - roe_dir: 0=down, 1=flat, 2=up
  - rev_dir: 0=down, 1=flat, 2=up
  - mscore_dir: 0=up(risky), 1=flat, 2=down(safe)
  - health_score: regression 0~100
"""
import torch
import torch.nn as nn


class SelfAttention(nn.Module):
    def __init__(self, hidden_dim):
        super().__init__()
        self.query = nn.Linear(hidden_dim, hidden_dim)
        self.key = nn.Linear(hidden_dim, hidden_dim)
        self.value = nn.Linear(hidden_dim, hidden_dim)
        self.scale = torch.sqrt(torch.FloatTensor([hidden_dim]))

    def forward(self, x):
        # x: [batch, seq_len, hidden_dim]
        Q = self.query(x)
        K = self.key(x)
        V = self.value(x)
        scores = torch.matmul(Q, K.transpose(-2, -1)) / self.scale.to(x.device)
        attn = torch.softmax(scores, dim=-1)
        out = torch.matmul(attn, V)
        return out, attn


class FinancialLSTM(nn.Module):
    def __init__(self, input_dim=16, hidden_dim=64, num_layers=2, dropout=0.2):
        super().__init__()
        self.lstm = nn.LSTM(
            input_dim, hidden_dim, num_layers,
            batch_first=True, dropout=dropout, bidirectional=True
        )
        attn_dim = hidden_dim * 2
        self.attention = SelfAttention(attn_dim)
        self.fc = nn.Sequential(
            nn.Linear(attn_dim, attn_dim // 2),
            nn.ReLU(),
            nn.Dropout(dropout),
        )
        self.head_roe = nn.Linear(attn_dim // 2, 3)
        self.head_rev = nn.Linear(attn_dim // 2, 3)
        self.head_mscore = nn.Linear(attn_dim // 2, 3)
        self.head_health = nn.Linear(attn_dim // 2, 1)

    def forward(self, x):
        # x: [batch, seq_len, input_dim]
        lstm_out, _ = self.lstm(x)  # [batch, seq_len, hidden*2]
        attn_out, attn_weights = self.attention(lstm_out)
        # mean over sequence
        pooled = attn_out.mean(dim=1)  # [batch, hidden*2]
        features = self.fc(pooled)
        roe_logits = self.head_roe(features)
        rev_logits = self.head_rev(features)
        mscore_logits = self.head_mscore(features)
        health = torch.sigmoid(self.head_health(features)) * 100.0
        return {
            'roe_dir': roe_logits,
            'rev_dir': rev_logits,
            'mscore_dir': mscore_logits,
            'health_score': health,
            'attn_weights': attn_weights,
        }


def build_model(input_dim=16):
    return FinancialLSTM(input_dim=input_dim)
