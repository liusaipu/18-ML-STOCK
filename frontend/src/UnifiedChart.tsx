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

// 图表配置
const chartHeight = 200
const chartWidthPercent = 0.85 // 85%宽度，留出空间给右轴
const chartColors = {
  up: '#ef4444',
  down: '#22c55e',
  grid: 'rgba(148, 163, 184, 0.1)',
  text: '#94a3b8',
  border: 'rgba(148, 163, 184, 0.2)',
}

// 统一的右轴宽度配置
const rightPriceScaleOptions = {
  borderColor: chartColors.border,
  minimumWidth: 60, // 固定最小宽度
}

// 时间轴配置
const timeScaleOptions = {
  borderColor: chartColors.border,
  timeVisible: false,
  visible: false,
  fixLeftEdge: false, // 允许滚动到边缘外
  fixRightEdge: false,
  rightOffset: 12, // 右侧留白
  barSpacing: 6, // 默认柱间距
  minBarSpacing: 2, // 最小柱间距（允许缩放到更小）
}

export function UnifiedChart({ code }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [data, setData] = useState<KlineData[]>([])
  const [loading, setLoading] = useState(false)
  
  // 图表引用
  const priceChartRef = useRef<IChartApi | null>(null)
  const macdChartRef = useRef<IChartApi | null>(null)
  const rsiChartRef = useRef<IChartApi | null>(null)
  const bollChartRef = useRef<IChartApi | null>(null)

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
    if (!containerRef.current || data.length === 0) return

    const container = containerRef.current
    const width = container.clientWidth * chartWidthPercent

    // 公共配置
    const commonOptions = {
      width,
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: chartColors.text,
        fontSize: 11,
      },
      grid: {
        vertLines: { color: chartColors.grid },
        horzLines: { color: chartColors.grid },
      },
      crosshair: { mode: CrosshairMode.Magnet },
      rightPriceScale: rightPriceScaleOptions,
      timeScale: timeScaleOptions,
      // 启用滚动和缩放
      handleScroll: {
        mouseWheel: false, // 滚轮用于缩放
        pressedMouseMove: true, // 按下鼠标拖动
        horzTouchDrag: true,
        vertTouchDrag: false,
      },
      handleScale: {
        axisPressedMouseMove: {
          time: true,
          price: false,
        },
        mouseWheel: true, // 滚轮缩放
        pinch: true,
      },
    }

    // 1. K线 + 成交量图
    const priceChart = createChart(container, {
      ...commonOptions,
      height: chartHeight,
    })
    priceChartRef.current = priceChart

    // 先添加成交量（在底层）
    const volSeries = priceChart.addSeries(HistogramSeries, {
      color: '#3b82f6',
      priceFormat: { type: 'volume' },
      priceScaleId: 'volume', // 使用独立的volume轴
    })
    // 成交量只在底部10%区域显示
    volSeries.priceScale().applyOptions({ scaleMargins: { top: 0.90, bottom: 0 } })

    // 再添加K线（在上层，主价格轴）
    const candleSeries = priceChart.addSeries(CandlestickSeries, {
      upColor: chartColors.up, downColor: chartColors.down,
      borderUpColor: chartColors.up, borderDownColor: chartColors.down,
      wickUpColor: chartColors.up, wickDownColor: chartColors.down,
    })

    // 2. MACD图
    const macdChart = createChart(container, {
      ...commonOptions,
      height: chartHeight * 0.7,
    })
    macdChartRef.current = macdChart

    const difSeries = macdChart.addSeries(LineSeries, { color: '#f59e0b', lineWidth: 2 })
    const deaSeries = macdChart.addSeries(LineSeries, { color: '#3b82f6', lineWidth: 2 })
    const macdHist = macdChart.addSeries(HistogramSeries, { color: '#10b981' })

    // 3. RSI图
    const rsiChart = createChart(container, {
      ...commonOptions,
      height: chartHeight * 0.6,
    })
    rsiChartRef.current = rsiChart

    const rsiSeries = rsiChart.addSeries(LineSeries, { color: '#8b5cf6', lineWidth: 2 })

    // 4. 布林带图
    const bollChart = createChart(container, {
      ...commonOptions,
      height: chartHeight + 50, // 增加更多高度给时间轴
      timeScale: { 
        ...commonOptions.timeScale,
        visible: true, // 只有底部显示时间轴
        timeVisible: false, // 只显示日期，不显示时间
        tickMarkMaxCharacterLength: 10,
        borderVisible: true,
      },
      // 底部图表需要更大的底部边距给时间轴标签
      layout: {
        background: { type: ColorType.Solid, color: 'transparent' },
        textColor: chartColors.text,
        fontSize: 11,
        attributionLogo: false,
      },
    })
    bollChartRef.current = bollChart

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
    const charts = [priceChart, macdChart, rsiChart, bollChart]
    let isSyncing = false
    
    charts.forEach((chart, index) => {
      chart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
        if (isSyncing || !range) return
        isSyncing = true
        charts.forEach((otherChart, otherIndex) => {
          if (index !== otherIndex) {
            otherChart.timeScale().setVisibleLogicalRange(range)
          }
        })
        isSyncing = false
      })
    })

    // 自适应并缩小50%以显示更多数据
    priceChart.timeScale().fitContent()
    // 缩小图表，显示更多数据（增加逻辑范围）
    setTimeout(() => {
      const logicalRange = priceChart.timeScale().getVisibleLogicalRange()
      if (logicalRange) {
        // 扩大逻辑范围50%，显示更多数据
        const newRange = {
          from: logicalRange.from,
          to: logicalRange.to + (logicalRange.to - logicalRange.from) * 0.5
        }
        charts.forEach(c => c.timeScale().setVisibleLogicalRange(newRange))
      }
    }, 100)

    // 响应式
    const handleResize = () => {
      const newWidth = container.clientWidth * chartWidthPercent
      charts.forEach(c => c.applyOptions({ width: newWidth }))
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      priceChart.remove()
      macdChart.remove()
      rsiChart.remove()
      bollChart.remove()
    }
  }, [data])

  if (loading) {
    return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>加载图表数据中...</div>
  }

  if (data.length === 0) {
    return <div style={{ padding: 40, textAlign: 'center', color: '#64748b' }}>暂无K线数据</div>
  }

  return (
    <div style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
      <div ref={containerRef} style={{ width: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        {/* 图表容器，子元素由useEffect创建 */}
      </div>
    </div>
  )
}
