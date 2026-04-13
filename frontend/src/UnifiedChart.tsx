import { useEffect, useRef, useState } from 'react'
import * as echarts from 'echarts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines } from '../wailsjs/go/main/App'

type KlineData = downloader.KlineData

interface Props {
  code: string
}

// ECharts 颜色配置
const colors = {
  up: '#ef4444',
  down: '#22c55e',
  ma5: '#eab308',
  ma10: '#3b82f6',
  macd: '#f59e0b',
  signal: '#3b82f6',
  histPositive: '#ef4444',
  histNegative: '#22c55e',
  rsi: '#8b5cf6',
  bbUpper: '#ef4444',
  bbMid: '#f59e0b',
  bbLower: '#10b981',
}

export function UnifiedChart({ code }: Props) {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)
  const [data, setData] = useState<KlineData[]>([])
  const [loading, setLoading] = useState(false)

  // 加载数据
  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetStockKlines(code)
      .then((list) => setData(list || []))
      .catch(() => setData([]))
      .finally(() => setLoading(false))
  }, [code])

  // 计算技术指标
  const calculateIndicators = (data: KlineData[]) => {
    const closes = data.map(d => d.close)
    
    // EMA
    const calcEMA = (arr: number[], period: number) => {
      const k = 2 / (period + 1)
      const ema: number[] = []
      for (let i = 0; i < arr.length; i++) {
        if (i === 0) ema.push(arr[0])
        else ema.push(arr[i] * k + ema[i - 1] * (1 - k))
      }
      return ema
    }
    
    // MACD
    const ema12 = calcEMA(closes, 12)
    const ema26 = calcEMA(closes, 26)
    const dif = ema12.map((v, i) => v - ema26[i])
    const dea = calcEMA(dif, 9)
    const hist = dif.map((v, i) => v - dea[i])
    
    // RSI
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
    
    // Bollinger Bands
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
    
    return { dif, dea, hist, rsi, bbUpper, bbMid, bbLower }
  }

  // 初始化图表
  useEffect(() => {
    if (!chartRef.current || data.length === 0) return

    // 销毁旧实例
    if (chartInstanceRef.current) {
      chartInstanceRef.current.dispose()
    }

    const chart = echarts.init(chartRef.current, 'dark', {
      renderer: 'canvas',
    })
    chartInstanceRef.current = chart

    const { dif, dea, hist, rsi, bbUpper, bbMid, bbLower } = calculateIndicators(data)
    const dates = data.map(d => d.time)

    const option: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      animation: false,
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
        backgroundColor: 'rgba(30, 41, 59, 0.9)',
        borderColor: 'rgba(148, 163, 184, 0.3)',
        textStyle: { color: '#94a3b8' },
      },
      // 使用 grid 布局，四个图表上下排列
      grid: [
        { left: '5%', right: '5%', top: '3%', height: '22%' },  // K线
        { left: '5%', right: '5%', top: '28%', height: '14%' }, // MACD
        { left: '5%', right: '5%', top: '45%', height: '12%' }, // RSI
        { left: '5%', right: '5%', top: '60%', height: '22%' }, // 布林带
      ],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: false, axisLine: { onZero: false }, splitLine: { show: false }, gridIndex: 0 },
        { type: 'category', data: dates, boundaryGap: false, axisLine: { onZero: false }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 1 },
        { type: 'category', data: dates, boundaryGap: false, axisLine: { onZero: false }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 2 },
        { type: 'category', data: dates, boundaryGap: false, axisLine: { onZero: false }, splitLine: { show: false }, gridIndex: 3 },
      ],
      yAxis: [
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.1)' } }, gridIndex: 0 },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.1)' } }, gridIndex: 1 },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.1)' } }, max: 100, min: 0, gridIndex: 2 },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.1)' } }, gridIndex: 3 },
      ],
      // 数据缩放：统一控制所有图表
      dataZoom: [
        { type: 'inside', xAxisIndex: [0, 1, 2, 3], start: 50, end: 100 },
        { type: 'slider', xAxisIndex: [0, 1, 2, 3], start: 50, end: 100, height: 20, bottom: 10 },
      ],
      series: [
        // K线 + 成交量
        {
          name: 'K线',
          type: 'candlestick',
          data: data.map(d => [d.open, d.close, d.low, d.high]),
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
          name: '成交量',
          type: 'bar',
          data: data.map((d) => ({
            value: d.volume,
            itemStyle: {
              color: d.close >= d.open ? 'rgba(239, 68, 68, 0.3)' : 'rgba(34, 197, 94, 0.3)',
            },
          })),
          xAxisIndex: 0,
          yAxisIndex: 0,
        },
        // MACD
        { name: 'DIF', type: 'line', data: dif, smooth: true, lineStyle: { color: colors.macd }, symbol: 'none', xAxisIndex: 1, yAxisIndex: 1 },
        { name: 'DEA', type: 'line', data: dea, smooth: true, lineStyle: { color: colors.signal }, symbol: 'none', xAxisIndex: 1, yAxisIndex: 1 },
        { 
          name: 'MACD', type: 'bar', data: hist.map(v => ({
            value: v,
            itemStyle: { color: v >= 0 ? colors.histPositive : colors.histNegative },
          })),
          xAxisIndex: 1, yAxisIndex: 1,
        },
        // RSI
        { name: 'RSI', type: 'line', data: rsi, smooth: true, lineStyle: { color: colors.rsi, width: 2 }, symbol: 'none', xAxisIndex: 2, yAxisIndex: 2 },
        // 布林带
        { name: '上轨', type: 'line', data: bbUpper, smooth: true, lineStyle: { color: colors.bbUpper }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3 },
        { name: '中轨', type: 'line', data: bbMid, smooth: true, lineStyle: { color: colors.bbMid, width: 2 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3 },
        { name: '下轨', type: 'line', data: bbLower, smooth: true, lineStyle: { color: colors.bbLower }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3 },
      ],
    }

    chart.setOption(option)

    // 响应式
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
    <div style={{ width: '100%', height: '700px' }}>
      <div ref={chartRef} style={{ width: '100%', height: '100%' }} />
    </div>
  )
}
