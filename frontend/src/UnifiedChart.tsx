import { useEffect, useRef, useState } from 'react'
import {
  createChart,
  CandlestickSeries,
  HistogramSeries,
  LineSeries,
  ColorType,
  CrosshairMode,
  type IChartApi,
  type Time,
} from 'lightweight-charts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines } from '../wailsjs/go/main/App'

type KlineData = downloader.KlineData

interface Props {
  code: string
}

// 技术指标计算函数
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

const chartColors = {
  up: '#ef4444',
  down: '#22c55e',
  grid: 'rgba(148, 163, 184, 0.1)',
  text: '#94a3b8',
  border: 'rgba(148, 163, 184, 0.2)',
}

// 固定图表尺寸配置
const CHART_WIDTH = 800 // 固定宽度
const PRICE_AXIS_WIDTH = 80 // 右轴固定宽度

export function UnifiedChart({ code }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [data, setData] = useState<KlineData[]>([])
  const [loading, setLoading] = useState(false)
  
  // 图表容器引用
  const priceContainerRef = useRef<HTMLDivElement>(null)
  const macdContainerRef = useRef<HTMLDivElement>(null)
  const rsiContainerRef = useRef<HTMLDivElement>(null)
  const bollContainerRef = useRef<HTMLDivElement>(null)
  
  // 图表实例引用
  const chartsRef = useRef<IChartApi[]>([])

  // 加载数据
  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetStockKlines(code)
      .then((list) => setData(list || []))
      .catch(() => setData([]))
      .finally(() => setLoading(false))
  }, [code])

  // 创建图表
  useEffect(() => {
    if (!priceContainerRef.current || data.length === 0) return

    // 清理旧图表
    chartsRef.current.forEach(c => c.remove())
    chartsRef.current = []

    // 统一的图表配置
    const createChartOptions = (height: number, showTimeScale: boolean = false) => ({
      width: CHART_WIDTH,
      height,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: chartColors.text,
        fontSize: 11,
        attributionLogo: false,
      },
      grid: {
        vertLines: { color: chartColors.grid },
        horzLines: { color: chartColors.grid },
      },
      crosshair: { 
        mode: CrosshairMode.Magnet,
      },
      rightPriceScale: {
        borderColor: chartColors.border,
        visible: true,
        minimumWidth: PRICE_AXIS_WIDTH,
        scaleMargins: { top: 0.05, bottom: 0.05 },
      },
      leftPriceScale: { visible: false },
      timeScale: {
        borderColor: chartColors.border,
        visible: showTimeScale,
        timeVisible: false,
        tickMarkMaxCharacterLength: 8,
        fixLeftEdge: false,
        fixRightEdge: false,
        rightOffset: 0,
        barSpacing: 5,
        minBarSpacing: 2,
      },
      handleScroll: {
        mouseWheel: false,
        pressedMouseMove: true,
        horzTouchDrag: true,
        vertTouchDrag: false,
      },
      handleScale: {
        axisPressedMouseMove: { time: true, price: false },
        mouseWheel: true,
        pinch: true,
      },
    })

    // 1. K线图 (180px)
    const priceChart = createChart(priceContainerRef.current, createChartOptions(180))
    chartsRef.current.push(priceChart)

    const candleSeries = priceChart.addSeries(CandlestickSeries, {
      upColor: chartColors.up, downColor: chartColors.down,
      borderUpColor: chartColors.up, borderDownColor: chartColors.down,
      wickUpColor: chartColors.up, wickDownColor: chartColors.down,
    })

    const volSeries = priceChart.addSeries(HistogramSeries, {
      color: '#3b82f6',
      priceFormat: { type: 'volume' },
      priceScaleId: 'volume',
    })
    volSeries.priceScale().applyOptions({ 
      scaleMargins: { top: 0.90, bottom: 0 },
      minimumWidth: PRICE_AXIS_WIDTH,
    })

    // 2. MACD图 (120px)
    const macdChart = createChart(macdContainerRef.current!, createChartOptions(120))
    chartsRef.current.push(macdChart)

    const difSeries = macdChart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2 })
    const deaSeries = macdChart.addSeries(LineSeries, { color: '#3b82f6', lineWidth: 2 })
    const macdHist = macdChart.addSeries(HistogramSeries, { color: '#10b981' })

    // 3. RSI图 (100px)
    const rsiChart = createChart(rsiContainerRef.current!, createChartOptions(100))
    chartsRef.current.push(rsiChart)

    const rsiSeries = rsiChart.addSeries(LineSeries, { color: '#8b5cf6', lineWidth: 2 })

    // 4. 布林带图 (180px + 60px时间轴 = 240px)
    const bollChart = createChart(bollContainerRef.current!, createChartOptions(240, true))
    chartsRef.current.push(bollChart)

    const bbUpper = bollChart.addSeries(LineSeries, { color: '#ef4444', lineWidth: 1 })
    const bbMid = bollChart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2 })
    const bbLower = bollChart.addSeries(LineSeries, { color: '#10b981', lineWidth: 1 })

    // 设置数据
    const candleData = data.map(d => ({ 
      time: d.time as Time, 
      open: d.open, 
      high: d.high, 
      low: d.low, 
      close: d.close 
    }))
    const volData = data.map(d => ({ 
      time: d.time as Time, 
      value: d.volume, 
      color: d.close >= d.open ? 'rgba(239, 68, 68, 0.5)' : 'rgba(34, 197, 94, 0.5)' 
    }))

    candleSeries.setData(candleData)
    volSeries.setData(volData)

    const closes = data.map(d => d.close)
    const { dif, dea, hist } = calcMACD(closes)
    difSeries.setData(data.map((d, i) => ({ time: d.time as Time, value: dif[i] })))
    deaSeries.setData(data.map((d, i) => ({ time: d.time as Time, value: dea[i] })))
    macdHist.setData(data.map((d, i) => ({ 
      time: d.time as Time, 
      value: hist[i], 
      color: hist[i] >= 0 ? '#10b981' : '#ef4444' 
    })))

    const rsi = calcRSI(closes)
    rsiSeries.setData(data.slice(14).map((d, i) => ({ time: d.time as Time, value: rsi[i] })))

    const { upper, mid, lower } = calcBollinger(closes)
    bbUpper.setData(data.slice(19).map((d, i) => ({ time: d.time as Time, value: upper[i] })))
    bbMid.setData(data.slice(19).map((d, i) => ({ time: d.time as Time, value: mid[i] })))
    bbLower.setData(data.slice(19).map((d, i) => ({ time: d.time as Time, value: lower[i] })))

    // 联动缩放
    let isSyncing = false
    chartsRef.current.forEach((chart, index) => {
      chart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
        if (isSyncing || !range) return
        isSyncing = true
        chartsRef.current.forEach((otherChart, otherIndex) => {
          if (index !== otherIndex) {
            otherChart.timeScale().setVisibleLogicalRange(range)
          }
        })
        isSyncing = false
      })
    })

    // 初始视图
    chartsRef.current.forEach(chart => chart.timeScale().fitContent())

    return () => {
      chartsRef.current.forEach(c => c.remove())
      chartsRef.current = []
    }
  }, [data])

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>加载图表数据中...</div>
  }

  if (data.length === 0) {
    return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>暂无K线数据</div>
  }

  // CSS 强制对齐布局
  const chartContainerStyle: React.CSSProperties = {
    width: `${CHART_WIDTH}px`,
    margin: '0 auto',
  }

  const rowStyle: React.CSSProperties = {
    width: `${CHART_WIDTH}px`,
    display: 'flex',
    justifyContent: 'center',
  }

  return (
    <div ref={containerRef} style={{ width: '100%', overflowX: 'auto' }}>
      <div style={chartContainerStyle}>
        {/* K线图 */}
        <div style={rowStyle}>
          <div ref={priceContainerRef} style={{ width: `${CHART_WIDTH}px`, height: '180px' }} />
        </div>
        
        {/* MACD图 */}
        <div style={rowStyle}>
          <div ref={macdContainerRef} style={{ width: `${CHART_WIDTH}px`, height: '120px' }} />
        </div>
        
        {/* RSI图 */}
        <div style={rowStyle}>
          <div ref={rsiContainerRef} style={{ width: `${CHART_WIDTH}px`, height: '100px' }} />
        </div>
        
        {/* 布林带图 (含时间轴) */}
        <div style={rowStyle}>
          <div ref={bollContainerRef} style={{ width: `${CHART_WIDTH}px`, height: '240px' }} />
        </div>
      </div>
    </div>
  )
}
