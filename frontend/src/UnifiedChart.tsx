import { useEffect, useMemo, useRef, useState } from 'react'
import * as echarts from 'echarts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines } from '../wailsjs/go/main/App'

type KlineData = downloader.KlineData
type StockQuote = downloader.StockQuote

interface Props {
  code: string
  quote?: StockQuote
}

const colors = {
  up: '#ef4444',
  down: '#22c55e',
  ma5: '#fbbf24',
  ma10: '#60a5fa',
  ma30: '#a78bfa',
  ma60: '#f87171',
  macd: '#f59e0b',
  signal: '#3b82f6',
  histPositive: '#ef4444',
  histNegative: '#22c55e',
  rsi: '#8b5cf6',
  bbUpper: '#ef4444',
  bbMid: '#f59e0b',
  bbLower: '#10b981',
}

const DISPLAY_SIZE = 120

function calcEMA(arr: number[], period: number): (number | null)[] {
  const k = 2 / (period + 1)
  const ema: (number | null)[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i === 0) ema.push(arr[0])
    else ema.push(arr[i] * k + (ema[i - 1] as number) * (1 - k))
  }
  return ema
}

function calcMA(arr: number[], period: number): (number | null)[] {
  const ma: (number | null)[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i < period - 1) { ma.push(null); continue }
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
  const ma10 = calcMA(closes, 10)
  const ma30 = calcMA(closes, 30)
  const ma60 = calcMA(closes, 60)

  const ema12 = calcEMA(closes, 12)
  const ema26 = calcEMA(closes, 26)
  const dif: (number | null)[] = ema12.map((v, i) => (v == null || ema26[i] == null) ? null : v - ema26[i]!)
  const validDif = dif.filter((v): v is number => v != null)
  const validDea = calcEMA(validDif, 9)
  const dea: (number | null)[] = []
  let deaIdx = 0
  for (let i = 0; i < dif.length; i++) {
    if (dif[i] == null) dea.push(null)
    else dea.push(validDea[deaIdx++] ?? null)
  }
  const hist: (number | null)[] = dif.map((v, i) => (v == null || dea[i] == null) ? null : v - dea[i]!)

  const rsi: (number | null)[] = []
  for (let i = 0; i < closes.length; i++) {
    if (i < 14) { rsi.push(null); continue }
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

  const bbUpper: (number | null)[] = [], bbMid: (number | null)[] = [], bbLower: (number | null)[] = []
  for (let i = 0; i < closes.length; i++) {
    if (i < 19) { bbUpper.push(null); bbMid.push(null); bbLower.push(null); continue }
    const slice = closes.slice(i - 19, i + 1)
    const mean = slice.reduce((a, b) => a + b, 0) / 20
    const std = Math.sqrt(slice.reduce((sq, n) => sq + Math.pow(n - mean, 2), 0) / 20)
    bbMid.push(mean)
    bbUpper.push(mean + 2 * std)
    bbLower.push(mean - 2 * std)
  }

  return { dif, dea, hist, rsi, bbUpper, bbMid, bbLower, ma5, ma10, ma30, ma60 }
}

function fmt2(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(2)
}
function fmt3(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(3)
}
function fmt1(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(1)
}

export function UnifiedChart({ code, quote }: Props) {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)
  const [rawData, setRawData] = useState<KlineData[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetStockKlines(code)
      .then((list) => setRawData(list || []))
      .catch(() => setRawData([]))
      .finally(() => setLoading(false))
  }, [code])

  const data = useMemo(() => {
    if (rawData.length === 0) return []
    // 如果K线没有换手率且quote有流通市值，计算近似换手率
    const hasTurnover = rawData.some(d => d.turnoverRate > 0)
    if (hasTurnover || !quote || quote.circulatingMarketCap <= 0 || quote.currentPrice <= 0) {
      return rawData
    }
    const circulatingShares = quote.circulatingMarketCap / quote.currentPrice
    return rawData.map(d => ({
      ...d,
      turnoverRate: (d.volume * 100 / circulatingShares) * 100,
    }))
  }, [rawData, quote])

  useEffect(() => {
    if (!chartRef.current || data.length === 0) return

    if (chartInstanceRef.current) {
      chartInstanceRef.current.dispose()
    }

    const chart = echarts.init(chartRef.current, 'dark', { renderer: 'canvas' })
    chartInstanceRef.current = chart

    // 用全部数据计算指标，但只显示最近120条
    const { dif, dea, hist, rsi, bbUpper, bbMid, bbLower, ma5, ma10, ma30, ma60 } = calculateIndicators(data)

    const displayData = data.slice(-DISPLAY_SIZE)
    const padCount = DISPLAY_SIZE - displayData.length
    const dates = [...Array(padCount).fill(''), ...displayData.map(d => d.time)]

    const nullPad = Array(padCount).fill(null)
    const candleData = [...nullPad, ...displayData.map(d => [d.open, d.close, d.low, d.high])]
    const turnoverData = [...nullPad, ...displayData.map((d: KlineData) => ({
      value: d.turnoverRate,
      itemStyle: { color: d.close >= d.open ? 'rgba(239,68,68,0.35)' : 'rgba(34,197,94,0.35)' },
    }))]

    const sliceDisplay = (arr: (number | null)[]) => {
      const sliced = arr.slice(-DISPLAY_SIZE)
      return padArray(sliced, DISPLAY_SIZE)
    }

    const option: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      animation: false,
      legend: {
        data: ['K线', 'MA5', 'MA10', 'MA30', 'MA60'],
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
          label: { show: false },
        },
        backgroundColor: 'rgba(15, 23, 42, 0.95)',
        borderColor: 'rgba(148, 163, 184, 0.25)',
        borderWidth: 1,
        textStyle: { color: '#e2e8f0', fontSize: 12 },
        padding: 0,
        formatter: (params: any) => {
          if (!params || params.length === 0) return ''
          const date = params[0].axisValue || ''
          if (!date) return ''

          const leftItems: string[] = []
          const candle = params.find((p: any) => p.seriesName === 'K线')
          if (candle && candle.data) {
            const [o, c, l, h] = candle.data
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">开盘</span><span>${fmt2(o)}</span></div>`)
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">收盘</span><span>${fmt2(c)}</span></div>`)
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">最低</span><span>${fmt2(l)}</span></div>`)
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">最高</span><span>${fmt2(h)}</span></div>`)
          }
          params.filter((p: any) => ['MA5', 'MA10', 'MA30', 'MA60'].includes(p.seriesName)).forEach((p: any) => {
            const color = p.color || '#94a3b8'
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt2(p.value)}</span></div>`)
          })
          const turnover = params.find((p: any) => p.seriesName === '换手率')
          if (turnover) {
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px;margin-top:4px;border-top:1px solid rgba(148,163,184,0.12);padding-top:4px"><span style="color:#94a3b8">换手率</span><span>${turnover.value != null ? fmt2(turnover.value) + '%' : '-'}</span></div>`)
          }

          const rightItems: string[] = []
          const macdParams = params.filter((p: any) => ['DIF', 'DEA', 'MACD'].includes(p.seriesName))
          if (macdParams.length) {
            macdParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt3(p.value)}</span></div>`)
            })
          }
          const rsiP = params.find((p: any) => p.seriesName === 'RSI')
          if (rsiP) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${colors.rsi}">● RSI</span><span>${fmt1(rsiP.value)}</span></div>`)
          }
          const bbParams = params.filter((p: any) => ['上轨', '中轨', '下轨'].includes(p.seriesName))
          if (bbParams.length) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            bbParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt2(p.value)}</span></div>`)
            })
          }

          return `
            <div style="line-height:1.65;font-size:12px">
              <div style="font-weight:600;margin-bottom:6px;color:#f0f0f0;padding:10px 14px 0">${date}</div>
              <div style="display:flex;gap:14px;padding:0 14px 10px">
                <div style="min-width:110px">${leftItems.join('')}</div>
                <div style="min-width:110px">${rightItems.join('')}</div>
              </div>
            </div>
          `
        },
      },
      axisPointer: {
        link: [{ xAxisIndex: 'all' }],
        label: { show: false },
      },
      grid: [
        { left: 62, right: 16, top: 38, height: '260' },
        { left: 62, right: 16, top: '320', height: '50' },
        { left: 62, right: 16, top: '390', height: '42' },
        { left: 62, right: 16, top: '448', height: '65' },
      ],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10 }, splitLine: { show: false }, gridIndex: 0, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 1, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 2, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10 }, splitLine: { show: false }, gridIndex: 3, axisPointer: { label: { show: true, backgroundColor: '#3b82f6' } } },
      ],
      yAxis: [
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 0, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 5, name: 'K线', nameLocation: 'middle', nameGap: 40, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' } },
        { scale: true, splitLine: { show: false }, gridIndex: 0, position: 'right', axisLabel: { show: false }, axisLine: { show: false }, axisTick: { show: false }, max: (value: any) => value.max * 4 },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 1, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 3, name: 'MACD', nameLocation: 'middle', nameGap: 40, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, min: 0, max: 100, gridIndex: 2, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 2, name: 'RSI', nameLocation: 'middle', nameGap: 40, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 3, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8' }, splitNumber: 4, name: '布林带', nameLocation: 'middle', nameGap: 40, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' } },
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
          name: '换手率',
          type: 'bar',
          data: turnoverData,
          xAxisIndex: 0,
          yAxisIndex: 1,
        },
        { name: 'MA5', type: 'line', data: sliceDisplay(ma5), smooth: false, lineStyle: { color: colors.ma5, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'MA10', type: 'line', data: sliceDisplay(ma10), smooth: false, lineStyle: { color: colors.ma10, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'MA30', type: 'line', data: sliceDisplay(ma30), smooth: false, lineStyle: { color: colors.ma30, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'MA60', type: 'line', data: sliceDisplay(ma60), smooth: false, lineStyle: { color: colors.ma60, width: 1.5 }, symbol: 'none', xAxisIndex: 0, yAxisIndex: 0 },
        { name: 'DIF', type: 'line', data: sliceDisplay(dif), smooth: true, lineStyle: { color: colors.macd }, symbol: 'none', xAxisIndex: 1, yAxisIndex: 2 },
        { name: 'DEA', type: 'line', data: sliceDisplay(dea), smooth: true, lineStyle: { color: colors.signal }, symbol: 'none', xAxisIndex: 1, yAxisIndex: 2 },
        {
          name: 'MACD', type: 'bar', data: sliceDisplay(hist).map(v => ({
            value: v,
            itemStyle: { color: v != null && v >= 0 ? colors.histPositive : colors.histNegative },
          })),
          xAxisIndex: 1, yAxisIndex: 2,
        },
        { name: 'RSI', type: 'line', data: sliceDisplay(rsi), smooth: true, lineStyle: { color: colors.rsi, width: 2 }, symbol: 'none', xAxisIndex: 2, yAxisIndex: 3, connectNulls: false },
        { name: '上轨', type: 'line', data: sliceDisplay(bbUpper), smooth: true, lineStyle: { color: colors.bbUpper }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 4, connectNulls: false },
        { name: '中轨', type: 'line', data: sliceDisplay(bbMid), smooth: true, lineStyle: { color: colors.bbMid, width: 2 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 4, connectNulls: false },
        { name: '下轨', type: 'line', data: sliceDisplay(bbLower), smooth: true, lineStyle: { color: colors.bbLower }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 4, connectNulls: false },
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
