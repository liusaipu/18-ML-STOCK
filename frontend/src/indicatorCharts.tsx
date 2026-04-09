import { useEffect, useRef, useState } from 'react'
import {
  createChart,
  LineSeries,
  HistogramSeries,
  ColorType,
  type IChartApi,
  type ISeriesApi,
} from 'lightweight-charts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines } from '../wailsjs/go/main/App'

type KlineData = downloader.KlineData

function useKlinesForChart() {
  const [data, setData] = useState<KlineData[]>([])
  const [loading, setLoading] = useState(false)

  const load = async (code: string) => {
    if (!code || data.length > 0) return
    setLoading(true)
    try {
      const list = await GetStockKlines(code)
      setData(list || [])
    } catch {
      setData([])
    } finally {
      setLoading(false)
    }
  }

  return { data, loading, load }
}

// ---------------- EMA helper ----------------
function calcEMA(values: number[], period: number): number[] {
  const k = 2 / (period + 1)
  const ema: number[] = []
  for (let i = 0; i < values.length; i++) {
    if (i === 0) {
      ema.push(values[0])
    } else {
      ema.push(values[i] * k + ema[i - 1] * (1 - k))
    }
  }
  return ema
}

// ---------------- MACD ----------------
export function MACDChart({ code, width = 500, height = 220 }: { code: string; width?: number; height?: number }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const difRef = useRef<ISeriesApi<'Line'> | null>(null)
  const deaRef = useRef<ISeriesApi<'Line'> | null>(null)
  const histRef = useRef<ISeriesApi<'Histogram'> | null>(null)
  const { data, load } = useKlinesForChart()

  useEffect(() => {
    load(code)
  }, [code])

  useEffect(() => {
    if (!containerRef.current || data.length === 0) return

    if (!chartRef.current) {
      const chart = createChart(containerRef.current, {
        width,
        height,
        layout: { background: { type: ColorType.Solid, color: 'transparent' }, textColor: '#94a3b8' },
        grid: { vertLines: { color: 'rgba(148,163,184,0.1)' }, horzLines: { color: 'rgba(148,163,184,0.1)' } },
        rightPriceScale: { borderColor: 'rgba(148,163,184,0.2)' },
        timeScale: { borderColor: 'rgba(148,163,184,0.2)', timeVisible: false },
      })
      chartRef.current = chart

      difRef.current = chart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2 })
      deaRef.current = chart.addSeries(LineSeries, { color: '#3b82f6', lineWidth: 2 })
      histRef.current = chart.addSeries(HistogramSeries, {
        color: '#10b981',
      })
    }

    const closes = data.map((d) => d.close)
    const ema12 = calcEMA(closes, 12)
    const ema26 = calcEMA(closes, 26)
    const dif = ema12.map((v, i) => v - ema26[i])
    const dea = calcEMA(dif, 9)
    const hist = dif.map((v, i) => v - dea[i])

    const difData = data.map((d, i) => ({ time: d.time, value: dif[i] }))
    const deaData = data.map((d, i) => ({ time: d.time, value: dea[i] }))
    const histData = data.map((d, i) => ({ time: d.time, value: hist[i], color: hist[i] >= 0 ? '#10b981' : '#ef4444' }))

    difRef.current?.setData(difData as any)
    deaRef.current?.setData(deaData as any)
    histRef.current?.setData(histData as any)
    chartRef.current?.timeScale().fitContent()

    return () => {
      chartRef.current?.remove()
      chartRef.current = null
      difRef.current = null
      deaRef.current = null
      histRef.current = null
    }
  }, [data, width, height])

  if (data.length === 0) {
    return <div style={{ width, height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#64748b', fontSize: 12 }}>加载MACD数据中...</div>
  }

  return <div ref={containerRef} style={{ width, height }} />
}

// ---------------- RSI ----------------
export function RSIChart({ code, width = 500, height = 220 }: { code: string; width?: number; height?: number }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const rsiRef = useRef<ISeriesApi<'Line'> | null>(null)
  const { data, load } = useKlinesForChart()

  useEffect(() => {
    load(code)
  }, [code])

  useEffect(() => {
    if (!containerRef.current || data.length === 0) return

    if (!chartRef.current) {
      const chart = createChart(containerRef.current, {
        width,
        height,
        layout: { background: { type: ColorType.Solid, color: 'transparent' }, textColor: '#94a3b8' },
        grid: { vertLines: { color: 'rgba(148,163,184,0.1)' }, horzLines: { color: 'rgba(148,163,184,0.1)' } },
        rightPriceScale: { borderColor: 'rgba(148,163,184,0.2)' },
        timeScale: { borderColor: 'rgba(148,163,184,0.2)', timeVisible: false },
      })
      chartRef.current = chart
      rsiRef.current = chart.addSeries(LineSeries, { color: '#8b5cf6', lineWidth: 2 })
    }

    const closes = data.map((d) => d.close)
    const rsi: number[] = []
    const period = 14
    let gains = 0
    let losses = 0
    for (let i = 1; i <= period; i++) {
      const diff = closes[i] - closes[i - 1]
      if (diff >= 0) gains += diff
      else losses += -diff
    }
    let avgGain = gains / period
    let avgLoss = losses / period
    rsi.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))

    for (let i = period + 1; i < closes.length; i++) {
      const diff = closes[i] - closes[i - 1]
      const gain = diff > 0 ? diff : 0
      const loss = diff < 0 ? -diff : 0
      avgGain = (avgGain * (period - 1) + gain) / period
      avgLoss = (avgLoss * (period - 1) + loss) / period
      rsi.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
    }

    const rsiData = data.slice(period).map((d, i) => ({ time: d.time, value: rsi[i] }))
    rsiRef.current?.setData(rsiData as any)
    chartRef.current?.timeScale().fitContent()

    return () => {
      chartRef.current?.remove()
      chartRef.current = null
      rsiRef.current = null
    }
  }, [data, width, height])

  if (data.length === 0) {
    return <div style={{ width, height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#64748b', fontSize: 12 }}>加载RSI数据中...</div>
  }

  return <div ref={containerRef} style={{ width, height }} />
}

// ---------------- Bollinger Bands ----------------
export function BollingerChart({ code, width = 500, height = 220 }: { code: string; width?: number; height?: number }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const upperRef = useRef<ISeriesApi<'Line'> | null>(null)
  const midRef = useRef<ISeriesApi<'Line'> | null>(null)
  const lowerRef = useRef<ISeriesApi<'Line'> | null>(null)
  const { data, load } = useKlinesForChart()

  useEffect(() => {
    load(code)
  }, [code])

  useEffect(() => {
    if (!containerRef.current || data.length === 0) return

    if (!chartRef.current) {
      const chart = createChart(containerRef.current, {
        width,
        height,
        layout: { background: { type: ColorType.Solid, color: 'transparent' }, textColor: '#94a3b8' },
        grid: { vertLines: { color: 'rgba(148,163,184,0.1)' }, horzLines: { color: 'rgba(148,163,184,0.1)' } },
        rightPriceScale: { borderColor: 'rgba(148,163,184,0.2)' },
        timeScale: { borderColor: 'rgba(148,163,184,0.2)', timeVisible: false },
      })
      chartRef.current = chart
      upperRef.current = chart.addSeries(LineSeries, { color: '#ef4444', lineWidth: 1 })
      midRef.current = chart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2 })
      lowerRef.current = chart.addSeries(LineSeries, { color: '#10b981', lineWidth: 1 })
    }

    const period = 20
    const closes = data.map((d) => d.close)
    const upper: number[] = []
    const mid: number[] = []
    const lower: number[] = []

    for (let i = period - 1; i < closes.length; i++) {
      const slice = closes.slice(i - period + 1, i + 1)
      const mean = slice.reduce((a, b) => a + b, 0) / period
      const std = Math.sqrt(slice.reduce((sq, n) => sq + Math.pow(n - mean, 2), 0) / period)
      mid.push(mean)
      upper.push(mean + 2 * std)
      lower.push(mean - 2 * std)
    }

    const sliceData = data.slice(period - 1)
    upperRef.current?.setData(sliceData.map((d, i) => ({ time: d.time, value: upper[i] })) as any)
    midRef.current?.setData(sliceData.map((d, i) => ({ time: d.time, value: mid[i] })) as any)
    lowerRef.current?.setData(sliceData.map((d, i) => ({ time: d.time, value: lower[i] })) as any)
    chartRef.current?.timeScale().fitContent()

    return () => {
      chartRef.current?.remove()
      chartRef.current = null
      upperRef.current = null
      midRef.current = null
      lowerRef.current = null
    }
  }, [data, width, height])

  if (data.length === 0) {
    return <div style={{ width, height, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#64748b', fontSize: 12 }}>加载布林带数据中...</div>
  }

  return <div ref={containerRef} style={{ width, height }} />
}
