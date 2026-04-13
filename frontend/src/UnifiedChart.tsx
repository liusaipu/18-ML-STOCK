import { useEffect, useRef, useState } from 'react'
import {
  createChart,
  CandlestickSeries,
  HistogramSeries,
  LineSeries,
  ColorType,
  CrosshairMode,
  type IChartApi,
} from 'lightweight-charts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines } from '../wailsjs/go/main/App'

type KlineData = downloader.KlineData

interface Props {
  code: string
  height?: number
}

function calcEMA(values: number[], period: number): number[] {
  const k = 2 / (period + 1)
  const ema: number[] = []
  for (let i = 0; i < values.length; i++) {
    if (i === 0) ema.push(values[0])
    else ema.push(values[i] * k + ema[i - 1] * (1 - k))
  }
  return ema
}

function calcRSI(closes: number[], period: number = 14): number[] {
  const rsi: number[] = []
  let gains = 0, losses = 0
  for (let i = 1; i <= period; i++) {
    const diff = closes[i] - closes[i - 1]
    if (diff >= 0) gains += diff
    else losses += -diff
  }
  let avgGain = gains / period, avgLoss = losses / period
  rsi.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
  for (let i = period + 1; i < closes.length; i++) {
    const diff = closes[i] - closes[i - 1]
    const gain = diff > 0 ? diff : 0
    const loss = diff < 0 ? -diff : 0
    avgGain = (avgGain * (period - 1) + gain) / period
    avgLoss = (avgLoss * (period - 1) + loss) / period
    rsi.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
  }
  return rsi
}

function calcBollinger(closes: number[], period: number = 20) {
  const upper: number[] = [], mid: number[] = [], lower: number[] = []
  for (let i = period - 1; i < closes.length; i++) {
    const slice = closes.slice(i - period + 1, i + 1)
    const mean = slice.reduce((a, b) => a + b, 0) / period
    const std = Math.sqrt(slice.reduce((sq, n) => sq + Math.pow(n - mean, 2), 0) / period)
    mid.push(mean)
    upper.push(mean + 2 * std)
    lower.push(mean - 2 * std)
  }
  return { upper, mid, lower }
}

function calcMACD(closes: number[]) {
  const ema12 = calcEMA(closes, 12)
  const ema26 = calcEMA(closes, 26)
  const dif = ema12.map((v, i) => v - ema26[i])
  const dea = calcEMA(dif, 9)
  const hist = dif.map((v, i) => v - dea[i])
  return { dif, dea, hist }
}

export function UnifiedChart({ code, height = 800 }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const [data, setData] = useState<KlineData[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetStockKlines(code)
      .then((list) => setData((list || []).slice(-120)))
      .catch(() => setData([]))
      .finally(() => setLoading(false))
  }, [code])

  useEffect(() => {
    if (!containerRef.current || data.length === 0) return

    const chart = createChart(containerRef.current, {
      autoSize: true,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: '#94a3b8',
      },
      grid: {
        vertLines: { color: 'rgba(148, 163, 184, 0.1)' },
        horzLines: { color: 'rgba(148, 163, 184, 0.1)' },
      },
      crosshair: { mode: CrosshairMode.Magnet },
      rightPriceScale: { borderColor: 'rgba(148, 163, 184, 0.2)' },
      timeScale: { borderColor: 'rgba(148, 163, 184, 0.2)', timeVisible: false },
    })
    chartRef.current = chart

    // K线
    const candleSeries = chart.addSeries(CandlestickSeries, {
      upColor: '#ef4444', downColor: '#22c55e',
      borderUpColor: '#ef4444', borderDownColor: '#22c55e',
      wickUpColor: '#ef4444', wickDownColor: '#22c55e',
    })

    // 成交量
    const volSeries = chart.addSeries(HistogramSeries, {
      color: '#3b82f6', priceFormat: { type: 'volume' }, priceScaleId: 'volume',
    })
    volSeries.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0 } })

    // 布林带
    const bbUpper = chart.addSeries(LineSeries, { color: '#ef4444', lineWidth: 1 })
    const bbMid = chart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2 })
    const bbLower = chart.addSeries(LineSeries, { color: '#10b981', lineWidth: 1 })

    // MACD
    const difSeries = chart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2, priceScaleId: 'macd' })
    const deaSeries = chart.addSeries(LineSeries, { color: '#3b82f6', lineWidth: 2, priceScaleId: 'macd' })
    const macdHist = chart.addSeries(HistogramSeries, { color: '#10b981', priceScaleId: 'macd' })
    chart.priceScale('macd').applyOptions({ scaleMargins: { top: 0.7, bottom: 0 } })

    // RSI
    const rsiSeries = chart.addSeries(LineSeries, { color: '#8b5cf6', lineWidth: 2, priceScaleId: 'rsi' })
    chart.priceScale('rsi').applyOptions({ scaleMargins: { top: 0.9, bottom: 0.7 } })

    const candleData = data.map(d => ({ time: d.time, open: d.open, high: d.high, low: d.low, close: d.close }))
    const volData = data.map(d => ({ time: d.time, value: d.volume, color: d.close >= d.open ? 'rgba(239, 68, 68, 0.5)' : 'rgba(34, 197, 94, 0.5)' }))
    
    candleSeries.setData(candleData)
    volSeries.setData(volData)

    const closes = data.map(d => d.close)

    const { dif, dea, hist } = calcMACD(closes)
    difSeries.setData(data.map((d, i) => ({ time: d.time, value: dif[i] })))
    deaSeries.setData(data.map((d, i) => ({ time: d.time, value: dea[i] })))
    macdHist.setData(data.map((d, i) => ({ time: d.time, value: hist[i], color: hist[i] >= 0 ? '#10b981' : '#ef4444' })))

    const rsi = calcRSI(closes)
    rsiSeries.setData(data.slice(14).map((d, i) => ({ time: d.time, value: rsi[i] })))

    const { upper, mid, lower } = calcBollinger(closes)
    bbUpper.setData(data.slice(19).map((d, i) => ({ time: d.time, value: upper[i] })))
    bbMid.setData(data.slice(19).map((d, i) => ({ time: d.time, value: mid[i] })))
    bbLower.setData(data.slice(19).map((d, i) => ({ time: d.time, value: lower[i] })))

    chart.timeScale().fitContent()

    return () => {
      chart.remove()
      chartRef.current = null
    }
  }, [data])

  if (loading) {
    return <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#64748b' }}>加载图表数据中...</div>
  }

  if (data.length === 0) {
    return <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#64748b' }}>暂无K线数据</div>
  }

  return <div ref={containerRef} style={{ width: '100%', height }} />
}
