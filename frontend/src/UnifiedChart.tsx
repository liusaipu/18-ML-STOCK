import { useEffect, useRef, useState } from 'react'
import * as echarts from 'echarts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines } from '../wailsjs/go/main/App'

type KlineData = downloader.KlineData

interface Props {
  code: string
}

const colors = {
  up: '#ef4444',
  down: '#22c55e',
  ma5: '#fbbf24',
  ma30: '#60a5fa',
  ma180: '#a78bfa',
  ma250: '#f87171',
  macd: '#f59e0b',
  signal: '#3b82f6',
  histPositive: '#ef4444',
  histNegative: '#22c55e',
  rsi: '#8b5cf6',
  bbUpper: '#ef4444',
  bbMid: '#f59e0b',
  bbLower: '#10b981',
}

const WINDOW_SIZE = 120 // 约6个月交易日

function calcEMA(arr: number[], period: number): number[] {
  const k = 2 / (period + 1)
  const ema: number[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i === 0) ema.push(arr[0])
    else ema.push(arr[i] * k + ema[i - 1] * (1 - k))
  }
  return ema
}

function calcMA(arr: number[], period: number): number[] {
  const ma: number[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i < period - 1) { ma.push(arr[i]); continue }
    let sum = 0
    for (let j = i - period + 1; j <= i; j++) sum += arr[j]
    ma.push(sum / period)
  }
  return ma
}

function padArray<T>(arr: T[], size: number): (T | null)[] {
  const padCount = size - arr.length
  if (padCount <= 0) return arr
  return [...Array(padCount).fill(null), ...arr]
}

function calculateIndicators(data: KlineData[]) {
  const closes = data.map(d => d.close)
  const ma5 = calcMA(closes, 5)
  const ma30 = calcMA(closes, 30)
  const ma180 = calcMA(closes, 180)
  const ma250 = calcMA(closes, 250)

  const ema12 = calcEMA(closes, 12)
  const ema26 = calcEMA(closes, 26)
  const dif = ema12.map((v, i) => v - ema26[i])
  const dea = calcEMA(dif, 9)
  const hist = dif.map((v, i) => v - dea[i])

  const rsi: number[] = []
  for (let i = 0; i < closes.length; i++) {
    if (i < 14) { rsi.push(50); continue }
    let gains = 0, losses = 0
    for (let j = i - 13; j <= i; j++) {
      const diff = closes[j] - closes[j - 1]
      if (diff >= 0) gains += diff
      else losses += -diff
    }
    const avgGain = gains / 14
    const avgLoss = losses / 14
    rsi.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
  }

  const bbUpper: number[] = [], bbMid: number[] = [], bbLower: number[] = []
  for (let i = 0; i < closes.length; i++) {
    if (i < 19) { bbUpper.push(closes[i]); bbMid.push(closes[i]); bbLower.push(closes[i]); continue }
    const slice = closes.slice(i - 19, i + 1)
    const mean = slice.reduce((a, b) => a + b, 0) / 20
    const std = Math.sqrt(slice.reduce((sq, n) => sq + Math.pow(n - mean, 2), 0) / 20)
    bbMid.push(mean)
    bbUpper.push(mean + 2 * std)
    bbLower.push(mean - 2 * std)
  }

  return { dif, dea, hist, rsi, bbUpper, bbMid, bbLower, ma5, ma30, ma180, ma250 }
}

function formatTooltipValue(name: string, value: number): string {
  if (name.includes('MA')) return Number(value).toFixed(2)
  if (name === 'RSI') return Number(value).toFixed(1)
  if (['DIF', 'DEA', 'MACD'].includes(name)) return Number(value).toFixed(3)
  if (['上轨', '中轨', '下轨'].includes(name)) return Number(value).toFixed(2)
  if (name === '换手率') return Number(value).toFixed(2) + '%'
  if (name === '成交量') return Number(value).toFixed(0)
  return String(value)
}

export function UnifiedChart({ code }: Props) {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)
  const [data, setData] = useState<KlineData[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetStockKlines(code)
      .then((list) => setData(list || []))
      .catch(() => setData([]))
      .finally(() => setLoading(false))
  }, [code])

  useEffect(() => {
    if (!chartRef.current || data.length === 0) return

    if (chartInstanceRef.current) {
      chartInstanceRef.current.dispose()
    }

    const chart = echarts.init(chartRef.current, 'dark', { renderer: 'canvas' })
    chartInstanceRef.current = chart

    // 固定窗口：最近120个交易日
    const recentData = data.slice(-WINDOW_SIZE)
    const padCount = WINDOW_SIZE - recentData.length
    const dates = [...Array(padCount).fill(''), ...recentData.map(d => d.time)]

    const { dif, dea, hist, rsi, bbUpper, bbMid, bbLower, ma5, ma30, ma180, ma250 } = calculateIndicators(recentData)

    // 判断是否有换手率数据
    const hasTurnover = recentData.some(d => d.turnoverRate > 0)
    const turnoverLabel = hasTurnover ? '换手率' : '成交量'

    // 补null让数据右对齐
    const nullPad = Array(padCount).fill(null)
    const candleData = [...nullPad, ...recentData.map(d => [d.open, d.close, d.low, d.high])]
    const turnoverData = [...nullPad, ...recentData.map(d => ({
      value: hasTurnover ? d.turnoverRate : d.volume,
      itemStyle: { color: d.close >= d.open ? 'rgba(239,68,68,0.35)' : 'rgba(34,197,94,0.35)' },
    }))]

    const pad = (arr: number[]) => padArray(arr, WINDOW_SIZE)

    const option: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      animation: false,
      legend: {
        data: ['K线', 'MA5', 'MA30', 'MA180', 'MA250'],
        top: 8,
        right: 10,
        textStyle: { color: '#94a3b8', fontSize: 11 },
        itemStyle: { borderWidth: 0 },
        itemGap: 8,
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          link: [{ xAxisIndex: 'all' }] as any,
          label: { backgroundColor: '#3b82f6' },
        },
        backgroundColor: 'rgba(15, 23, 42, 0.95)',
        borderColor: 'rgba(148, 163, 184, 0.25)',
        borderWidth: 1,
        textStyle: { color: '#e2e8f0', fontSize: 12 },
        padding: [10, 14],
        formatter: (params: any) => {
          if (!params || params.length === 0) return ''
          const date = params[0].axisValue || ''
          if (!date) return ''

          const rows: string[] = [`<div style="font-weight:600;margin-bottom:6px;color:#f0f0f0">${date}</div>`]

          // K线 + 均线 + 换手率
          const klineGroup: string[] = []
          const candle = params.find((p: any) => p.seriesName === 'K线')
          if (candle && candle.data) {
            const [o, c, l, h] = candle.data
            klineGroup.push(`<div style="display:flex;justify-content:space-between;gap:20px"><span style="color:#94a3b8">open</span><span>${Number(o).toFixed(2)}</span></div>`)
            klineGroup.push(`<div style="display:flex;justify-content:space-between;gap:20px"><span style="color:#94a3b8">close</span><span>${Number(c).toFixed(2)}</span></div>`)
            klineGroup.push(`<div style="display:flex;justify-content:space-between;gap:20px"><span style="color:#94a3b8">low</span><span>${Number(l).toFixed(2)}</span></div>`)
            klineGroup.push(`<div style="display:flex;justify-content:space-between;gap:20px"><span style="color:#94a3b8">high</span><span>${Number(h).toFixed(2)}</span></div>`)
          }
          params.filter((p: any) => ['MA5', 'MA30', 'MA180', 'MA250'].includes(p.seriesName)).forEach((p: any) => {
            const color = p.color || '#94a3b8'
            klineGroup.push(`<div style="display:flex;justify-content:space-between;gap:20px"><span style="color:${color}">● ${p.seriesName}</span><span>${Number(p.value).toFixed(2)}</span></div>`)
          })
          const turnover = params.find((p: any) => p.seriesName === turnoverLabel)
          if (turnover && turnover.value != null) {
            klineGroup.push(`<div style="display:flex;justify-content:space-between;gap:20px"><span style="color:#94a3b8">${turnoverLabel}</span><span>${formatTooltipValue(turnoverLabel, turnover.value)}</span></div>`)
          }
          if (klineGroup.length) rows.push(klineGroup.join(''))

          // MACD
          const macdParams = params.filter((p: any) => ['DIF', 'DEA', 'MACD'].includes(p.seriesName))
          if (macdParams.length) {
            rows.push('<div style="border-top:1px solid rgba(148,163,184,0.15);margin:6px 0;padding-top:4px;font-size:11px;color:#94a3b8">MACD</div>')
            macdParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rows.push(`<div style="display:flex;justify-content:space-between;gap:20px;font-size:11px"><span style="color:${color}">● ${p.seriesName}</span><span>${formatTooltipValue(p.seriesName, p.value)}</span></div>`)
            })
          }

          // RSI
          const rsiP = params.find((p: any) => p.seriesName === 'RSI')
          if (rsiP && rsiP.value != null) {
            rows.push('<div style="border-top:1px solid rgba(148,163,184,0.15);margin:6px 0;padding-top:4px;font-size:11px;color:#94a3b8">RSI</div>')
            rows.push(`<div style="display:flex;justify-content:space-between;gap:20px;font-size:11px"><span style="color:${colors.rsi}">● RSI</span><span>${formatTooltipValue('RSI', rsiP.value)}</span></div>`)
          }

          // 布林带
          const bbParams = params.filter((p: any) => ['上轨', '中轨', '下轨'].includes(p.seriesName))
          if (bbParams.length) {
            rows.push('<div style="border-top:1px solid rgba(148,163,184,0.15);margin:6px 0;padding-top:4px;font-size:11px;color:#94a3b8">布林带</div>')
            bbParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rows.push(`<div style="display:flex;justify-content:space-between;gap:20px;font-size:11px"><span style="color:${color}">● ${p.seriesName}</span><span>${formatTooltipValue(p.seriesName, p.value)}</span></div>`)
            })
          }

          return `<div style="line-height:1.7">${rows.join('')}</div>`
        },
      },
      axisPointer: {
        link: [{ xAxisIndex: 'all' }],
        label: { backgroundColor: '#3b82f6' },
      },
      grid: [
        { left: 52, right: 16, top: 38, height: '260' },
        { left: 52, right: 16, top: '320', height: '50' },
        { left: 52, right: 16, top: '390', height: '42' },
        { left: 52, right: 16, top: '448', height: '65' },
      ],
      graphic: [
        { type: 'text', left: 10, top: 18, style: { text: 'K线', fill: '#94a3b8', fontSize: 11 } },
        { type: 'text', left: 10, top: 314, style: { text: 'MACD', fill: '#94a3b8', fontSize: 11 } },
        { type: 'text', left: 10, top: 384, style: { text: 'RSI', fill: '#94a3b8', fontSize: 11 } },
        { type: 'text', left: 10, top: 442, style: { text: '布林带', fill: '#94a3b8', fontSize: 11 } },
      ],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10 }, splitLine: { show: false }, gridIndex: 0 },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 1 },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 2 },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10 }, splitLine: { show: false }, gridIndex: 3 },
      ],
      yAxis: [
        // K线 左轴
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 0, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 5 },
        // K线 右轴（换手率/成交量）
        { scale: true, splitLine: { show: false }, gridIndex: 0, position: 'right', axisLabel: { show: false }, axisLine: { show: false }, axisTick: { show: false }, max: (value: any) => value.max * 4 },
        // MACD
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 1, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 3 },
        // RSI
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, min: 0, max: 100, gridIndex: 2, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 2 },
        // 布林带
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 3, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 4 },
      ],
      dataZoom: [
        { type: 'inside', xAxisIndex: [0, 1, 2, 3], start: 0, end: 100, zoomLock: true },
      ],
      series: [
        {
          name: 'K线',
          type: 'candlestick',
          data: candleData,
          itemStyle: {
            color: colors.up,
            color0: colors.down,
            borderColor: colors.up,
            borderColor0: colors.down,
          },
          xAxisIndex: 0,
          yAxisIndex: 0,
        },
        {
          name: turnoverLabel,
          type: 'bar',
          data: turnoverData,
          xAxisIndex: 0,
          yAxisIndex: 1,
        },
        { name: 'MA5', type: 'line', data: pad(ma5), smooth: false, lineStyle: { color: colors.ma5, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'MA30', type: 'line', data: pad(ma30), smooth: false, lineStyle: { color: colors.ma30, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'MA180', type: 'line', data: pad(ma180), smooth: false, lineStyle: { color: colors.ma180, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'MA250', type: 'line', data: pad(ma250), smooth: false, lineStyle: { color: colors.ma250, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'DIF', type: 'line', data: pad(dif), smooth: true, lineStyle: { color: colors.macd }, symbol: 'none', xAxisIndex: 1, yAxisIndex: 2 },
        { name: 'DEA', type: 'line', data: pad(dea), smooth: true, lineStyle: { color: colors.signal }, symbol: 'none', xAxisIndex: 1, yAxisIndex: 2 },
        {
          name: 'MACD', type: 'bar', data: pad(hist).map(v => ({
            value: v,
            itemStyle: { color: v != null && v >= 0 ? colors.histPositive : colors.histNegative },
          })),
          xAxisIndex: 1, yAxisIndex: 2,
        },
        { name: 'RSI', type: 'line', data: pad(rsi), smooth: true, lineStyle: { color: colors.rsi, width: 2 }, symbol: 'none', xAxisIndex: 2, yAxisIndex: 3 },
        { name: '上轨', type: 'line', data: pad(bbUpper), smooth: true, lineStyle: { color: colors.bbUpper }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 4 },
        { name: '中轨', type: 'line', data: pad(bbMid), smooth: true, lineStyle: { color: colors.bbMid, width: 2 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 4 },
        { name: '下轨', type: 'line', data: pad(bbLower), smooth: true, lineStyle: { color: colors.bbLower }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 4 },
      ],
    }

    chart.setOption(option)

    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.dispose()
      chartInstanceRef.current = null
    }
  }, [data])

  if (loading) return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>加载图表数据中...</div>
  if (data.length === 0) return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>暂无K线数据</div>

  return (
    <div style={{ width: '100%', height: '560px' }}>
      <div ref={chartRef} style={{ width: '100%', height: '100%' }} />
    </div>
  )
}
