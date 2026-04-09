import { useState, useEffect, useMemo, useRef, useCallback } from 'react'
import './App.css'
import { STOCKS } from './stocks'
import { KlineChart } from './KlineChart'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeSlug from 'rehype-slug'
import rehypeRaw from 'rehype-raw'
import { pinyin } from 'pinyin-pro'

function formatAmount(val: number, unit: string): string {
  if (!val || val <= 0) return '-'
  const abs = Math.abs(val)
  if (abs >= 1e8) return `${(val / 1e8).toFixed(2)} 亿${unit}`
  if (abs >= 1e4) return `${(val / 1e4).toFixed(2)} 万${unit}`
  return `${val.toFixed(0)} ${unit}`
}

function Collapsible({ title, children, defaultExpanded = false }: { title: React.ReactNode; children: React.ReactNode; defaultExpanded?: boolean }) {
  const [expanded, setExpanded] = useState(defaultExpanded)
  return (
    <div className="collapsible-section">
      <div className="collapsible-header" onClick={() => setExpanded(!expanded)}>
        <span className="collapsible-title">{title}</span>
        <span className="collapsible-toggle">{expanded ? '收起' : '展开'}</span>
      </div>
      {expanded && <div className="collapsible-body">{children}</div>}
    </div>
  )
}
import {
  GetWatchlist,
  GetWatchlistActivity,
  AddToWatchlist,
  RemoveFromWatchlist,
  ReorderWatchlist,
  ImportFinancialReports,
  DownloadReports,
  AnalyzeStock,
  AnalyzeStockWithRIM,
  CheckAnalysisCache,
  DownloadReport,
  DeleteReport,
  GetReportHistory,
  GetReport,
  GetStockDataHistory,
  GetStockProfile,
  RefreshStockProfile,
  GetComparables,
  AddComparable,
  RemoveComparable,
  DownloadComparableReports,
  GetStockQuote,
  GetStockKlines,
  GetStockConcepts,
  ExportCurrentFinancialData,
  ExportHistoricalFinancialData,
} from '../wailsjs/go/main/App'
import type { main, analyzer, downloader } from '../wailsjs/go/models'

type Stock = main.StockInfo
type WatchlistItem = main.WatchlistItem
type ImportResult = main.ImportResult
type AnalysisReport = analyzer.AnalysisReport
type StepResult = analyzer.StepResult
type DownloadResult = main.DownloadResult
type HistoryMeta = main.HistoryMeta
type StockProfile = main.StockProfile
type StockQuote = downloader.StockQuote
type KlineData = downloader.KlineData

function getStepValue(steps: StepResult[], stepNum: number, year: string, key: string): number {
  const step = steps.find((s) => s.stepNum === stepNum)
  if (!step || !step.yearlyData || !step.yearlyData[year]) return 0
  return Number(step.yearlyData[year][key] || 0)
}

function extractHighlightsAndRisks(report: AnalysisReport) {
  const latest = report.years[0]
  if (!latest) return { highlights: [], risks: [] }
  const steps = report.stepResults || []

  const roe = getStepValue(steps, 16, latest, 'roe')
  const gm = getStepValue(steps, 10, latest, 'grossMargin')
  const growth = getStepValue(steps, 9, latest, 'growthRate')
  const pg = getStepValue(steps, 16, latest, 'profitGrowth')
  const ms = getStepValue(steps, 8, latest, 'MScore')
  const dr = getStepValue(steps, 3, latest, 'debtRatio')
  const cr = getStepValue(steps, 15, latest, 'cashRatio')

  const highlights: string[] = []
  const risks: string[] = []

  if (roe >= 15) highlights.push('ROE 优秀，资本回报能力强')
  else risks.push('ROE 低于 15%，资本回报能力有待提升')

  if (gm >= 40) highlights.push('高毛利率，定价权稳固')
  else risks.push('毛利率未达 40%，产品竞争力一般')

  if (dr <= 40) highlights.push('低负债率，财务结构稳健')
  else if (dr > 60) risks.push('负债率超过 60%，偿债压力偏大')

  if (ms <= -2.22) highlights.push('M-Score 安全，财报可信度高')
  else if (ms > -1.78) risks.push('M-Score 异常，财报操纵风险较高')
  else risks.push('M-Score 偏高，需警惕财报操纵嫌疑')

  if (growth >= 10) highlights.push('营收稳健增长')
  else if (growth < 0) risks.push('营收负增长，成长性承压')

  if (pg >= 10) highlights.push('净利润持续增长')
  else if (pg < 0) risks.push('净利润下滑，盈利能力减弱')

  if (cr >= 100) highlights.push('经营现金流充沛，盈利质量高')
  else if (cr > 0) risks.push('现金流含金量不足')

  return { highlights, risks }
}

function formatFilename(dateStr: string) {
  // 2006-01-02_15-04-05_分析报告.md -> 2006-01-02 15:04
  const parts = dateStr.split('_')
  if (parts.length >= 2) {
    return `${parts[0]} ${parts[1].replace(/-/g, ':')}`
  }
  return dateStr
}

function formatTimestamp(iso: string) {
  try {
    const d = new Date(iso)
    return d.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return iso
  }
}

function App() {
  const [watchlist, setWatchlist] = useState<WatchlistItem[]>([])
  const [selectedCode, setSelectedCode] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [showDropdown, setShowDropdown] = useState(false)
  const [loading, setLoading] = useState(false)
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const saved = localStorage.getItem('theme')
    return saved === 'light' ? 'light' : 'dark'
  })
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [downloadResult, setDownloadResult] = useState<DownloadResult | null>(null)
  const [downloading, setDownloading] = useState(false)
  const [report, setReport] = useState<AnalysisReport | null>(null)
  const [snapshots, setSnapshots] = useState<Record<string, AnalysisReport>>({})
  const [analyzing, setAnalyzing] = useState(false)
  const [analyzeProgress, setAnalyzeProgress] = useState(0)
  const [historyFiles, setHistoryFiles] = useState<string[]>([])
  const [viewingHistory, setViewingHistory] = useState<string | null>(null)
  const [historyContent, setHistoryContent] = useState<string>('')
  const [dataHistory, setDataHistory] = useState<HistoryMeta[]>([])
  const [profile, setProfile] = useState<StockProfile | null>(null)
  const [comparables, setComparables] = useState<string[]>([])
  const [appliedComparables, setAppliedComparables] = useState<string[]>([])
  const [compQuery, setCompQuery] = useState('')
  const [showCompDropdown, setShowCompDropdown] = useState(false)
  const [compDownloading, setCompDownloading] = useState(false)
  const [concepts, setConcepts] = useState<downloader.StockConcepts | null>(null)
  const [quote, setQuote] = useState<StockQuote | null>(null)
  const [quoteError, setQuoteError] = useState<string>('')
  const [klines, setKlines] = useState<KlineData[]>([])
  const [klineError, setKlineError] = useState<string>('')
  const [activityMap, setActivityMap] = useState<Record<string, main.WatchlistActivitySummary>>({})
  const [activitySort, setActivitySort] = useState<'none' | 'desc' | 'asc'>('none')
  const [flashCode, setFlashCode] = useState<string | null>(null)
  const flashTimeoutRef = useRef<number | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const reportContentRef = useRef<HTMLDivElement>(null)
  const dragIndexRef = useRef<number | null>(null)
  const reportSearchRef = useRef<HTMLInputElement>(null)
  const reportMatchesRef = useRef<HTMLElement[]>([])
  const reportSearchIndexRef = useRef(0)
  const reportLastQueryRef = useRef('')
  const [traceDrawerOpen, setTraceDrawerOpen] = useState(false)
  const [currentTrace, setCurrentTrace] = useState<analyzer.CalcTrace | null>(null)
  const [traceList, setTraceList] = useState<analyzer.CalcTrace[]>([])
  const [forceAnalyzeOpen, setForceAnalyzeOpen] = useState(false)
  const [lastAnalysisAt, setLastAnalysisAt] = useState('')

  // RIM 参数弹窗状态
  const [showRIMModal, setShowRIMModal] = useState(false)
  const [rimBeta, setRimBeta] = useState(0.98)
  const [rimRf, setRimRf] = useState(1.83)
  const [rimRmRf, setRimRmRf] = useState(5.17)
  const [rimG, setRimG] = useState(5.0)
  const [rimEPS, setRimEPS] = useState<(number | string)[]>(['0', '0', '0', '0', '0', '0'])
  const [rimBPS0, setRimBPS0] = useState(0)
  const [rimPrice, setRimPrice] = useState(0)
  const [rimLoading, setRimLoading] = useState(false)
  const [rimProgress, setRimProgress] = useState(0)

  const tocSections = [
    { label: '模块1: 执行摘要', id: '模块1-执行摘要' },
    { label: '模块2: 换手率深度分析', id: '模块2-换手率深度分析' },
    { label: '模块3: 公司基本面分析', id: '模块3-公司基本面分析' },
    { label: '模块4: 行业横向对比分析', id: '模块4-行业横向对比分析' },
    { label: '模块5: 十五五政策匹配度评估', id: '模块5-十五五政策匹配度评估' },
    { label: '模块6: 实时行情数据', id: '模块6-实时行情数据' },
    { label: '模块7: 剩余收益模型估值(RIM)', id: '模块7-剩余收益模型估值rim' },
    { label: '模块8: A-Score 综合风险画像', id: '模块8-a-score-综合风险画像' },
    { label: '模块9: 技术面分析', id: '模块9-技术面分析' },
    { label: '模块10: ML机器学习预测', id: '模块10-ml机器学习预测' },
    { label: '模块11: 智能选股6大条件', id: '模块11-智能选股6大条件' },
    { label: '模块12: 芒格逆向思维检查', id: '模块12-芒格逆向思维检查' },
    { label: '模块13: 巴菲特-芒格投资检查清单', id: '模块13-巴菲特-芒格投资检查清单' },
    { label: '模块14: 社交媒体情绪监控', id: '模块14-社交媒体情绪监控' },
    { label: '模块15: 综合投资建议', id: '模块15-综合投资建议' },
    { label: '模块16: 结论与附录', id: '模块16-结论与附录' },
  ]

  const handleTocJump = (id: string) => {
    if (!reportContentRef.current || !id) return
    const el = reportContentRef.current.querySelector(`#${CSS.escape(id)}`) as HTMLElement | null
    if (el) {
      const container = reportContentRef.current
      const top = el.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop - 12
      container.scrollTo({ top, behavior: 'smooth' })
    }
  }

  const clearSearchHighlights = () => {
    if (!reportContentRef.current) return
    const container = reportContentRef.current.querySelector('.markdown-body')
    if (!container) return
    const highlights = container.querySelectorAll('span.search-highlight, span.search-highlight-active')
    highlights.forEach((span) => {
      const parent = span.parentNode
      if (parent) {
        parent.replaceChild(document.createTextNode(span.textContent || ''), span)
        parent.normalize()
      }
    })
    reportMatchesRef.current = []
    reportSearchIndexRef.current = 0
  }

  const buildSearchHighlights = (query: string): number => {
    if (!reportContentRef.current) return 0
    const container = reportContentRef.current.querySelector('.markdown-body')
    if (!container) return 0
    clearSearchHighlights()
    const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT)
    const matches: { node: Text; start: number; end: number }[] = []
    const lowerQuery = query.toLowerCase()
    let node: Node | null
    while ((node = walker.nextNode())) {
      const textNode = node as Text
      const text = textNode.textContent || ''
      const lowerText = text.toLowerCase()
      let idx = 0
      while ((idx = lowerText.indexOf(lowerQuery, idx)) !== -1) {
        matches.push({ node: textNode, start: idx, end: idx + query.length })
        idx += query.length
      }
    }
    // Process from end to start so node positions don't shift
    for (let i = matches.length - 1; i >= 0; i--) {
      const { node, start, end } = matches[i]
      const range = document.createRange()
      range.setStart(node, start)
      range.setEnd(node, end)
      const span = document.createElement('span')
      span.className = 'search-highlight'
      try {
        range.surroundContents(span)
        reportMatchesRef.current.unshift(span)
      } catch {
        // ignore ranges that span multiple elements
      }
    }
    return reportMatchesRef.current.length
  }

  const handleReportSearchKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return
    const query = e.currentTarget.value.trim()
    if (!query) {
      clearSearchHighlights()
      return
    }
    if (!displayContent) {
      alert('没有可搜索的内容')
      return
    }
    // If query changed, rebuild highlights
    if (reportLastQueryRef.current !== query) {
      reportLastQueryRef.current = query
      reportSearchIndexRef.current = 0
      const count = buildSearchHighlights(query)
      if (count === 0) {
        alert('没有匹配')
        return
      }
    }
    const matches = reportMatchesRef.current
    if (matches.length === 0) {
      const count = buildSearchHighlights(query)
      if (count === 0) {
        alert('没有匹配')
        return
      }
    }
    // Remove active class from previous
    matches.forEach((m) => (m.className = 'search-highlight'))
    // Scroll to current match
    const currentIdx = reportSearchIndexRef.current
    const current = matches[currentIdx]
    current.className = 'search-highlight search-highlight-active'
    current.scrollIntoView({ behavior: 'smooth', block: 'center' })
    if (currentIdx === matches.length - 1) {
      alert('已经达到最后一个匹配的字串')
      reportSearchIndexRef.current = 0
    } else {
      reportSearchIndexRef.current++
    }
  }

  // 左栏宽度可拖动调整
  const [sidebarWidth, setSidebarWidth] = useState(220)
  const [isResizing, setIsResizing] = useState(false)

  useEffect(() => {
    if (!isResizing) return
    const handleMouseMove = (e: MouseEvent) => {
      const newWidth = Math.min(Math.max(e.clientX, 180), 380)
      setSidebarWidth(newWidth)
    }
    const handleMouseUp = () => setIsResizing(false)
    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
    return () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }
  }, [isResizing])

  // 拼音首字母缓存
  const pinyinMap = useMemo(() => {
    const map = new Map<string, string>()
    STOCKS.forEach((s) => {
      try {
        const py = pinyin(s.name, { pattern: 'first', toneType: 'none', type: 'string' }).toLowerCase().replace(/\s+/g, '')
        map.set(s.code, py)
      } catch {
        map.set(s.code, '')
      }
    })
    return map
  }, [])

  // 初始化加载自选列表及活跃度
  useEffect(() => {
    GetWatchlist().then((list) => {
      setWatchlist(list || [])
    })
    GetWatchlistActivity().then((list) => {
      const map: Record<string, main.WatchlistActivitySummary> = {}
      ;(list || []).forEach((item) => {
        map[item.code] = item
      })
      setActivityMap(map)
    })
  }, [])

  // 自选股变化时刷新活跃度
  useEffect(() => {
    if (watchlist.length === 0) return
    GetWatchlistActivity().then((list) => {
      console.log('[GetWatchlistActivity] returned', list)
      const map: Record<string, main.WatchlistActivitySummary> = {}
      ;(list || []).forEach((item) => {
        map[item.code] = item
      })
      setActivityMap(map)
    }).catch((err) => {
      console.error('[GetWatchlistActivity] error', err)
    })
  }, [watchlist.length])

  // 搜索输入时，若已加自选中有匹配，则高亮并滚动
  useEffect(() => {
    const q = query.trim()
    if (!q) {
      setFlashCode(null)
      return
    }
    const lower = q.toLowerCase()
    const matched = watchlist.find(
      (s) => s.code.toLowerCase().includes(lower) || s.name.toLowerCase().includes(lower)
    )
    if (matched) {
      setFlashCode(matched.code)
      if (flashTimeoutRef.current) {
        window.clearTimeout(flashTimeoutRef.current)
      }
      flashTimeoutRef.current = window.setTimeout(() => {
        setFlashCode(null)
      }, 1500)
      // 等待 DOM 更新后滚动
      requestAnimationFrame(() => {
        const el = document.querySelector(`.watchlist li[data-code="${matched.code}"]`)
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
        }
      })
    } else {
      setFlashCode(null)
    }
  }, [query, watchlist])

  // 主题持久化
  useEffect(() => {
    localStorage.setItem('theme', theme)
    if (theme === 'light') {
      document.body.classList.add('light')
    } else {
      document.body.classList.remove('light')
    }
  }, [theme])

  // 本地搜索过滤：按代码、名称或拼音首字母匹配，最多10条
  const suggestions = useMemo(() => {
    const q = query.trim()
    if (!q) return []
    const lower = q.toLowerCase()
    return STOCKS.filter(
      (s) =>
        s.code.toLowerCase().includes(lower) ||
        s.name.toLowerCase().includes(lower) ||
        (pinyinMap.get(s.code) || '').includes(lower)
    ).slice(0, 10)
  }, [query, pinyinMap])

  const selectedStock = useMemo(
    () => watchlist.find((s) => s.code === selectedCode) || null,
    [selectedCode, watchlist]
  )

  const displayWatchlist = useMemo(() => {
    if (activitySort === 'none') return watchlist
    const list = [...watchlist]
    list.sort((a, b) => {
      const scoreA = activityMap[a.code]?.score ?? -1
      const scoreB = activityMap[b.code]?.score ?? -1
      if (activitySort === 'desc') return scoreB - scoreA
      return scoreA - scoreB
    })
    return list
  }, [watchlist, activityMap, activitySort])

  const currentSnapshot = selectedStock ? snapshots[selectedStock.code] : null
  const { highlights, risks } = useMemo(() => {
    if (!currentSnapshot) return { highlights: [], risks: [] }
    return extractHighlightsAndRisks(currentSnapshot)
  }, [currentSnapshot])

  const loadReportHistory = useCallback(async (code: string, autoLoadLatest = false) => {
    try {
      const files = await GetReportHistory(code)
      setHistoryFiles(files || [])
      if (autoLoadLatest && files && files.length > 0) {
        const latest = files[0]
        const content = await GetReport(code, latest)
        setHistoryContent(content)
        setViewingHistory(latest)
      }
    } catch {
      setHistoryFiles([])
    }
  }, [])

  const loadDataHistory = useCallback(async (code: string) => {
    try {
      const list = await GetStockDataHistory(code)
      setDataHistory(list || [])
    } catch {
      setDataHistory([])
    }
  }, [])

  const loadProfile = useCallback(async (code: string) => {
    try {
      const p = await GetStockProfile(code)
      setProfile(p || null)
    } catch {
      setProfile(null)
    }
  }, [])

  const handleRefreshProfile = async () => {
    if (!selectedStock) return
    try {
      const p = await RefreshStockProfile(selectedStock.code)
      setProfile(p || null)
    } catch (err: any) {
      alert('刷新基本信息失败: ' + String(err))
    }
  }

  const loadConcepts = useCallback(async (code: string) => {
    try {
      const c = await GetStockConcepts(code)
      setConcepts(c || null)
    } catch {
      setConcepts(null)
    }
  }, [])

  const loadComparables = useCallback(async (code: string) => {
    try {
      const list = await GetComparables(code)
      setComparables(list || [])
      setAppliedComparables(list || [])
    } catch {
      setComparables([])
      setAppliedComparables([])
    }
  }, [])

  const loadQuote = useCallback(async (code: string) => {
    try {
      setQuoteError('')
      const q = await GetStockQuote(code)
      setQuote(q || null)
    } catch (err: any) {
      setQuote(null)
      setQuoteError('行情获取失败，请检查网络')
      console.error('行情加载失败:', err)
    }
  }, [])

  const loadKlines = useCallback(async (code: string) => {
    try {
      setKlineError('')
      const list = await GetStockKlines(code)
      setKlines(list || [])
    } catch (err: any) {
      setKlines([])
      setKlineError('K线数据获取失败')
      console.error('K线加载失败:', err)
    }
  }, [])

  const handleSelectSuggestion = async (stock: Stock) => {
    setQuery('')
    setShowDropdown(false)
    setLoading(true)
    try {
      await AddToWatchlist(stock.code)
      const list = await GetWatchlist()
      setWatchlist(list || [])
      setSelectedCode(stock.code)
      setProfile(null)
      setQuote(null)
      setQuoteError('')
      setKlines([])
      setKlineError('')
      setDownloadResult(null)
      setReport(null)
      setViewingHistory(null)
      setHistoryContent('')
      await loadReportHistory(stock.code, true)
      await loadDataHistory(stock.code)
      await loadProfile(stock.code)
      await loadConcepts(stock.code)
      await loadComparables(stock.code)
      await loadQuote(stock.code)
      await loadKlines(stock.code)
    } catch (e) {
      alert(String(e))
    } finally {
      setLoading(false)
    }
  }

  const handleRemove = async (code: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setLoading(true)
    try {
      await RemoveFromWatchlist(code)
      const list = await GetWatchlist()
      setWatchlist(list || [])
      if (selectedCode === code) {
        setSelectedCode(null)
        setProfile(null)
        setQuote(null)
        setQuoteError('')
        setKlines([])
        setKlineError('')
        setImportResult(null)
        setDownloadResult(null)
        setReport(null)
        setViewingHistory(null)
        setHistoryContent('')
        setHistoryFiles([])
        setDataHistory([])
        setComparables([])
        setConcepts(null)
      }
    } catch (err) {
      alert(String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleImport = async () => {
    if (!selectedStock) return
    setLoading(true)
    try {
      const result = await ImportFinancialReports(selectedStock.code)
      setImportResult(result)
      if (result && result.success) {
        alert(`导入成功！\n${result.message}\n资产负债表年份: ${result.balanceSheet?.join(', ') || '无'}\n利润表年份: ${result.income?.join(', ') || '无'}\n现金流量表年份: ${result.cashFlow?.join(', ') || '无'}`)
        await loadDataHistory(selectedStock.code)
      } else {
        alert('导入失败')
      }
    } catch (err: any) {
      console.error('导入失败:', err)
      alert(String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleDownload = async () => {
    if (!selectedStock) return
    setDownloading(true)
    try {
      const result = await DownloadReports(selectedStock.code)
      setDownloadResult(result)
      if (result.success) {
        alert(result.message + '\n年份: ' + result.years.join(', '))
        await loadDataHistory(selectedStock.code)
      } else {
        alert('下载失败')
      }
    } catch (err: any) {
      console.error('下载失败:', err)
      const msg = err?.message || String(err)
      if (msg.includes('companyType') || msg.includes('未找到') || msg.includes('无数据') || msg.includes('无法确定')) {
        alert('下载失败：该股票财报暂不可从网络获取，建议手动导入CSV')
      } else if (msg.includes('timeout') || msg.includes('Timeout') || msg.includes('超时')) {
        alert('下载失败：网络超时，请稍后重试')
      } else {
        alert('下载失败：' + msg)
      }
    } finally {
      setDownloading(false)
    }
  }

  const handleExportCurrentData = async () => {
    if (!selectedStock) return
    try {
      await ExportCurrentFinancialData(selectedStock.code)
    } catch (err: any) {
      console.error('导出当前数据失败:', err)
      alert('导出失败: ' + String(err))
    }
  }

  const handleExportHistoryData = async (timestamp: string) => {
    if (!selectedStock) return
    try {
      await ExportHistoricalFinancialData(selectedStock.code, timestamp)
    } catch (err: any) {
      console.error('导出历史数据失败:', err)
      alert('导出失败: ' + String(err))
    }
  }

  const runAnalyze = async (overwriteLatest = false) => {
    if (!selectedStock) return
    setAnalyzing(true)
    setAnalyzeProgress(5)
    const interval = setInterval(() => {
      setAnalyzeProgress((p) => (p >= 90 ? 90 : p + 3))
    }, 400)
    try {
      const result = await AnalyzeStock(selectedStock.code, overwriteLatest)
      setReport(result)
      setViewingHistory(null)
      setHistoryContent('')
      if (result) {
        setSnapshots((prev) => ({ ...prev, [selectedStock.code]: result }))
      }
      setAppliedComparables(comparables)
      await loadReportHistory(selectedStock.code)
    } catch (err: any) {
      console.error('分析失败:', err)
      alert(String(err))
    } finally {
      clearInterval(interval)
      setAnalyzeProgress(100)
      setTimeout(() => {
        setAnalyzing(false)
        setAnalyzeProgress(0)
      }, 400)
    }
  }

  const handleAnalyze = async () => {
    if (!selectedStock) return
    let overwriteLatest = false
    try {
      const cache = await CheckAnalysisCache(selectedStock.code)
      if (cache?.unchanged) {
        setLastAnalysisAt(cache.lastAnalysisAt || '')
        setForceAnalyzeOpen(true)
        return
      }
      // 数据没变但可比公司变了：覆盖上次报告
      overwriteLatest = !cache?.dataChanged && !!cache?.comparablesChanged
    } catch (err: any) {
      console.error('检查分析缓存失败:', err)
    }
    await runAnalyze(overwriteLatest)
  }

  const openRIMModal = () => {
    if (!selectedStock) return
    // 优先用当前报告中的RIM数据预填充，否则用默认值
    const rim = report?.rim
    if (rim && rim.hasData) {
      setRimBeta(rim.beta ?? 0.98)
      setRimRf((rim.rf ?? 0.0183) * 100)
      setRimRmRf((rim.rmRf ?? 0.0517) * 100)
      setRimG((rim.params?.GTerminal ?? 0.05) * 100)
      setRimBPS0(rim.params?.BPS0 ?? 0)
      setRimPrice(rim.params?.CurrentPrice ?? 0)
      let eps: (number | string)[] = []
      if (rim.epsRaw && Object.keys(rim.epsRaw).length > 0) {
        const years = Object.keys(rim.epsRaw).sort()
        eps = years.slice(0, 6).map((y) => rim.epsRaw![y].toFixed(2))
      }
      const forecast = rim.params?.Forecast?.EPS?.slice(0, 6) || []
      while (eps.length < 6) {
        eps.push((forecast[eps.length] ?? 0).toFixed(2))
      }
      setRimEPS(eps)
    } else if (quote) {
      // 从行情推算默认值
      setRimBeta(0.98)
      setRimRf(1.83)
      setRimRmRf(5.17)
      setRimG(5.0)
      setRimBPS0(quote.pb > 0 ? quote.currentPrice / quote.pb : 0)
      setRimPrice(quote.currentPrice)
      setRimEPS([0, 0, 0, 0, 0, 0])
    }
    setShowRIMModal(true)
  }

  const handleAnalyzeWithRIM = async () => {
    if (!selectedStock) return
    setRimLoading(true)
    setRimProgress(5)
    const interval = setInterval(() => {
      setRimProgress((p) => (p >= 90 ? 90 : p + 3))
    }, 400)
    try {
      const params = {
        BPS0: rimBPS0,
        KE: rimRf / 100 + rimBeta * (rimRmRf / 100),
        GTerminal: rimG / 100,
        Forecast: { EPS: rimEPS.map(Number).filter((v) => v > 0), DPS: [] },
        CurrentPrice: rimPrice,
      }
      const rimData = {
        hasData: true,
        params,
        result: null as any,
        rf: rimRf / 100,
        beta: rimBeta,
        rmRf: rimRmRf / 100,
      }
      const rimJSON = JSON.stringify(rimData)
      const result = await AnalyzeStockWithRIM(selectedStock.code, false, rimJSON)
      setReport(result)
      setViewingHistory(null)
      setHistoryContent('')
      if (result) {
        setSnapshots((prev) => ({ ...prev, [selectedStock.code]: result }))
      }
      setAppliedComparables(comparables)
      await loadReportHistory(selectedStock.code)
      setShowRIMModal(false)
    } catch (err: any) {
      console.error('RIM分析失败:', err)
      alert(String(err))
    } finally {
      clearInterval(interval)
      setRimProgress(100)
      setTimeout(() => {
        setRimLoading(false)
        setRimProgress(0)
      }, 400)
    }
  }

  const handleReportDownload = async () => {
    if (!report || !selectedStock) return
    const content = viewingHistory ? historyContent : report.markdownContent
    try {
      await DownloadReport(selectedStock.code, content)
    } catch (err: any) {
      console.error('下载报告失败:', err)
      alert(String(err))
    }
  }

  const handleDeleteReport = async () => {
    if (!selectedStock || !displayContent) return
    let filename = viewingHistory
    if (!filename) {
      const files = await GetReportHistory(selectedStock.code)
      if (files.length === 0) {
        alert('没有可删除的报告')
        return
      }
      filename = files[0]
    }
    if (!confirm(`确定删除报告 ${filename} 吗？`)) return
    try {
      await DeleteReport(selectedStock.code, filename)
      await loadReportHistory(selectedStock.code)
      if (viewingHistory === filename) {
        setViewingHistory(null)
        setHistoryContent('')
      }
      if (!viewingHistory) {
        // 删除的是最新报告，清空当前展示
        setReport(null)
      }
    } catch (err: any) {
      console.error('删除报告失败:', err)
      alert('删除报告失败: ' + String(err))
    }
  }

  const compSuggestions = useMemo(() => {
    const q = compQuery.trim()
    if (!q) return []
    const lower = q.toLowerCase()
    return STOCKS.filter(
      (s) =>
        s.code !== selectedCode &&
        (
          s.code.toLowerCase().includes(lower) ||
          s.name.toLowerCase().includes(lower) ||
          (pinyinMap.get(s.code) || '').includes(lower)
        )
    ).slice(0, 10)
  }, [compQuery, selectedCode, pinyinMap])

  const handleAddComparable = async (stock: Stock) => {
    if (!selectedStock || stock.code === selectedStock.code) return
    try {
      await AddComparable(selectedStock.code, stock.code)
      const list = await GetComparables(selectedStock.code)
      setComparables(list || [])
      setCompQuery('')
      setShowCompDropdown(false)
    } catch (err: any) {
      alert(String(err))
    }
  }

  const handleRemoveComparable = async (code: string) => {
    if (!selectedStock) return
    try {
      await RemoveComparable(selectedStock.code, code)
      const list = await GetComparables(selectedStock.code)
      setComparables(list || [])
    } catch (err: any) {
      alert(String(err))
    }
  }

  const handleDownloadComparables = async () => {
    if (!selectedStock || comparables.length === 0) return
    setCompDownloading(true)
    try {
      const result = await DownloadComparableReports(selectedStock.code)
      if (result) {
        alert(result.message)
      }
    } catch (err: any) {
      console.error('下载可比公司财报失败:', err)
      alert(String(err))
    } finally {
      setCompDownloading(false)
    }
  }

  const handleAnalyzeWithComparables = async () => {
    if (!selectedStock || comparables.length === 0) return
    setCompDownloading(true)
    setAnalyzing(true)
    setAnalyzeProgress(5)
    const interval = setInterval(() => {
      setAnalyzeProgress((p) => (p >= 90 ? 90 : p + 3))
    }, 400)
    try {
      const downloadResult = await DownloadComparableReports(selectedStock.code)
      if (downloadResult && !downloadResult.success) {
        alert(downloadResult.message || '下载可比公司财报失败')
        return
      }
      const result = await AnalyzeStock(selectedStock.code, true)
      setReport(result)
      setViewingHistory(null)
      setHistoryContent('')
      if (result) {
        setSnapshots((prev) => ({ ...prev, [selectedStock.code]: result }))
      }
      setAppliedComparables(comparables)
      await loadReportHistory(selectedStock.code)
      setTimeout(() => {
        handleTocJump('模块4-行业横向对比分析')
      }, 150)
    } catch (err: any) {
      console.error('分析失败:', err)
      alert(String(err))
    } finally {
      clearInterval(interval)
      setAnalyzeProgress(100)
      setTimeout(() => {
        setCompDownloading(false)
        setAnalyzing(false)
        setAnalyzeProgress(0)
      }, 400)
    }
  }

  const handleHistoryChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const filename = e.target.value
    if (!filename || filename === '__latest__' || !selectedStock) {
      setViewingHistory(null)
      setHistoryContent('')
      setTimeout(() => {
        handleTocJump('核心指标一览')
      }, 150)
      return
    }
    try {
      const content = await GetReport(selectedStock.code, filename)
      setHistoryContent(content)
      setViewingHistory(filename)
    } catch (err: any) {
      alert('加载历史报告失败: ' + String(err))
    }
  }

  const displayContent = viewingHistory ? historyContent : report?.markdownContent

  function formatTraceValue(v: number): string {
    const abs = Math.abs(v)
    if (abs >= 1e8) return `${(v / 1e8).toFixed(2)} 亿元`
    if (abs >= 1e4) return `${(v / 1e4).toFixed(2)} 万元`
    return `${v.toFixed(0)} 元`
  }

  function formatTraceResult(v: number, indicator: string): string {
    if (indicator.includes('率') || indicator === 'ROE' || indicator === '毛利率') {
      return `${v.toFixed(2)}%`
    }
    return formatTraceValue(v)
  }

  // 切换报告时清除搜索高亮和 trace
  useEffect(() => {
    clearSearchHighlights()
    reportLastQueryRef.current = ''
    if (reportSearchRef.current) {
      reportSearchRef.current.value = ''
    }
    setTraceDrawerOpen(false)
    setCurrentTrace(null)
    setTraceList([])
  }, [displayContent])

  const markdownComponents = useMemo(() => ({
    span({ className, 'data-steps': dataSteps, children, ...props }: any) {
      if (className === 'trace-trigger' && dataSteps) {
        const stepNums = String(dataSteps)
          .split(',')
          .map((s: string) => parseInt(s.trim(), 10))
          .filter((n: number) => !isNaN(n))
        return (
          <button
            className="trace-trigger-btn"
            onClick={() => {
              const matched =
                report?.stepResults?.flatMap((step) =>
                  stepNums.includes(step.stepNum) && step.traces ? step.traces : []
                ) || []
              if (matched.length > 0) {
                setTraceList(matched)
                setCurrentTrace(matched[0])
                setTraceDrawerOpen(true)
              }
            }}
            title="查看计算过程"
          >
            {children}
          </button>
        )
      }
      return (
        <span className={className} {...props}>
          {children}
        </span>
      )
    },
  }), [report])

  return (
    <div className={`app ${theme}`}>
      {/* 主题切换按钮 */}
      <button
        className="theme-toggle"
        title={theme === 'dark' ? '切换浅色模式' : '切换深色模式'}
        onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
      >
        {theme === 'dark' ? '☀️' : '🌙'}
      </button>

      {/* 左栏：自选列表 */}
      <aside className="sidebar" style={{ width: sidebarWidth, minWidth: sidebarWidth }}>
        <div className="sidebar-header">
          <h2>自选股票</h2>
          <div className="search-box">
            <input
              ref={inputRef}
              type="text"
              placeholder="输入代码或名称..."
              value={query}
              disabled={loading}
              onChange={(e) => {
                setQuery(e.target.value)
                setShowDropdown(true)
              }}
              onFocus={() => setShowDropdown(true)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  setShowDropdown(false)
                }
              }}
            />
            {showDropdown && suggestions.length > 0 && (
              <ul className="dropdown">
                {suggestions.map((s) => (
                  <li
                    key={s.code}
                    onClick={() => handleSelectSuggestion(s)}
                    className="dropdown-item"
                  >
                    <span className="stock-code">{s.code}</span>
                    <span className="stock-name">{s.name}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        <div className="watchlist-header">
          <span className="watch-header-name">股票名称</span>
          <span
            className="watch-header-activity"
            title="点击排序"
            onClick={() => {
              setActivitySort((prev) => {
                if (prev === 'none') return 'desc'
                if (prev === 'desc') return 'asc'
                return 'none'
              })
            }}
          >
            活跃度
            {activitySort === 'desc' && ' ▼'}
            {activitySort === 'asc' && ' ▲'}
            {activitySort === 'none' && ' ⇅'}
          </span>
          <span className="watch-header-action" />
        </div>
        <ul className="watchlist">
          {displayWatchlist.map((s, idx) => {
            const act = activityMap[s.code]
            const scoreText = act ? Math.round(act.score).toString() : '-'
            return (
              <li
                key={s.code}
                data-code={s.code}
                draggable={activitySort === 'none'}
                className={`${selectedCode === s.code ? 'active' : ''}${flashCode === s.code ? ' flash-match' : ''}`}
                onDragStart={() => {
                  dragIndexRef.current = idx
                }}
                onDragOver={(e) => {
                  if (activitySort !== 'none') return
                  e.preventDefault()
                }}
                onDrop={(e) => {
                  if (activitySort !== 'none') return
                  e.preventDefault()
                  const fromIdx = dragIndexRef.current
                  dragIndexRef.current = null
                  if (fromIdx === null || fromIdx === idx) return
                  const newList = [...displayWatchlist]
                  const [moved] = newList.splice(fromIdx, 1)
                  newList.splice(idx, 0, moved)
                  setWatchlist(newList)
                  setActivitySort('none')
                  const codes = newList.map((i) => i.code)
                  ReorderWatchlist(codes).catch((err) => console.error('排序保存失败:', err))
                }}
                onClick={() => {
                  setSelectedCode(s.code)
                  setProfile(null)
                  setQuote(null)
                  setQuoteError('')
                  setKlines([])
                  setKlineError('')
                  setImportResult(null)
                  setDownloadResult(null)
                  setReport(null)
                  setViewingHistory(null)
                  setHistoryContent('')
                  setComparables([])
                  loadReportHistory(s.code, true)
                  loadDataHistory(s.code)
                  loadProfile(s.code)
                  loadConcepts(s.code)
                  loadComparables(s.code)
                  loadQuote(s.code)
                  loadKlines(s.code)
                }}
              >
                <span className="watch-drag-handle" title={activitySort === 'none' ? '拖动排序' : '排序中禁用拖动'}>☰</span>
                <div className="watch-info" title={`${s.name}(${s.code})`}>
                  {s.name}<span className="code-part">({s.code})</span>
                </div>
                <div className="watch-activity" title={act ? `${act.grade} · ${Math.round(act.score)}分` : ''}>
                  {scoreText}
                </div>
                <button
                  className="btn-remove"
                  title="移除"
                  onClick={(e) => handleRemove(s.code, e)}
                  disabled={loading}
                >
                  ×
                </button>
              </li>
            )
          })}
        </ul>

        <div className="watchlist-footer">
          共 {watchlist.length} / 100 只
        </div>
        <div
          className="sidebar-resizer"
          onMouseDown={() => setIsResizing(true)}
          title="拖动调整宽度"
        />
      </aside>

      {/* 中栏：股票信息 & 操作 */}
      <section className="info-panel">
        {selectedStock ? (
          <>
            <div className="stock-header">
              <h1>{selectedStock.name}<span className="stock-sub">{selectedStock.code}</span></h1>
            </div>

            <div className="stock-info-card">
              <div className="stock-info-grid">
                <div className="stock-info-item">
                  <span className="stock-info-label">所属行业</span>
                  <span className="stock-info-value">{profile?.industry || '--'}</span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">上市日期</span>
                  <span className="stock-info-value">{profile?.listingDate || '--'}</span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">
                    {profile?.controller ? (
                      <><strong>实控人</strong>/董事长</>
                    ) : profile?.chairman ? (
                      <>实控人/<strong>董事长</strong></>
                    ) : (
                      '实控人/董事长'
                    )}
                  </span>
                  <span className="stock-info-value">
                    {profile?.controller || profile?.chairman || '--'}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">籍属</span>
                  <span className="stock-info-value">
                    {profile?.chairmanNationality ? (
                      profile.chairmanNationality === '中国台湾' && profile?.politicalAffiliation ? (
                        <strong
                          style={{
                            color: profile.politicalAffiliation === 'blue' ? '#3b82f6' : '#22c55e',
                          }}
                        >
                          {profile.chairmanNationality}
                        </strong>
                      ) : (
                        profile.chairmanNationality
                      )
                    ) : (
                      '--'
                    )}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">总市值</span>
                  <span className="stock-info-value">
                    {(profile?.marketCap || quote?.marketCap)
                      ? `${(((profile?.marketCap || 0) > 0 ? profile!.marketCap : quote!.marketCap) / 1e8).toFixed(2)} 亿`
                      : '--'}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">市盈率 (PE)</span>
                  <span className="stock-info-value">
                    {profile?.pe
                      ? profile.pe.toFixed(2)
                      : quote?.pe
                      ? quote.pe.toFixed(2)
                      : '--'}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">市净率 (PB)</span>
                  <span className="stock-info-value">
                    {profile?.pb
                      ? profile.pb.toFixed(2)
                      : quote?.pb
                      ? quote.pb.toFixed(2)
                      : '--'}
                  </span>
                </div>
              </div>
              <div className="stock-info-footer">
                <span className="stock-info-time">
                  {profile?.updatedAt
                    ? `更新于: ${new Date(profile.updatedAt).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}`
                    : '暂无数据'}
                </span>
                <button className="stock-info-refresh" onClick={handleRefreshProfile} title="强制刷新">
                  刷新
                </button>
              </div>
            </div>

            <div className="actions">
              <button className="btn primary" onClick={handleDownload} disabled={downloading || loading}>
                {downloading ? (
                  '下载中...'
                ) : (
                  <span className="btn-content">
                    下载财报
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                      <polyline points="7 10 12 15 17 10" />
                      <line x1="12" y1="15" x2="12" y2="3" />
                    </svg>
                  </span>
                )}
              </button>
              <button className="btn primary" onClick={handleAnalyze} disabled={analyzing || downloading || loading}>
                {analyzing ? (
                  <>
                    <span className="btn-progress" style={{ width: `${analyzeProgress}%` }} />
                    <span style={{ position: 'relative', zIndex: 1 }}>分析中 {analyzeProgress}%</span>
                  </>
                ) : (
                  <span className="btn-content">
                    18步分析
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                      <line x1="4" y1="20" x2="4" y2="14" />
                      <line x1="8" y1="20" x2="8" y2="10" />
                      <line x1="12" y1="20" x2="12" y2="16" />
                      <path d="M16 12l4 4-4 4" />
                    </svg>
                  </span>
                )}
              </button>
            </div>
            <div className="actions-sub">
              <button className="btn-text" onClick={handleImport} disabled={loading}>
                {loading ? '处理中...' : '导入本地csv/excel财报'}
              </button>
            </div>

            {importResult && importResult.success && (
              <Collapsible title="📥 导入结果">
                <div className="import-summary" style={{ marginBottom: 0 }}>
                  <div className="import-row">
                    <span className="import-label">资产负债表</span>
                    <span className="import-years">
                      {importResult.balanceSheet?.length
                        ? `${importResult.balanceSheet.length} 年: ${importResult.balanceSheet.join(', ')}`
                        : '未导入'}
                    </span>
                  </div>
                  <div className="import-row">
                    <span className="import-label">利润表</span>
                    <span className="import-years">
                      {importResult.income?.length
                        ? `${importResult.income.length} 年: ${importResult.income.join(', ')}`
                        : '未导入'}
                    </span>
                  </div>
                  <div className="import-row">
                    <span className="import-label">现金流量表</span>
                    <span className="import-years">
                      {importResult.cashFlow?.length
                        ? `${importResult.cashFlow.length} 年: ${importResult.cashFlow.join(', ')}`
                        : '未导入'}
                    </span>
                  </div>
                </div>
              </Collapsible>
            )}

            {downloadResult && downloadResult.success && (
              <Collapsible title="⬇️ 下载结果">
                <div className="import-summary" style={{ marginBottom: 0 }}>
                  <div className="import-row">
                    <span className="import-label">网络下载</span>
                    <span className="import-years">
                      {downloadResult.years?.length
                        ? `${downloadResult.years.length} 年: ${downloadResult.years.join(', ')}`
                        : '无'}
                    </span>
                  </div>
                  {downloadResult.validation && downloadResult.validation.length > 0 && (
                    <div style={{ marginTop: 8 }}>
                      <div style={{ fontWeight: 600, marginBottom: 4 }}>数据校验：</div>
                      {downloadResult.validation.map((v, idx) => (
                        <div
                          key={idx}
                          style={{
                            fontSize: 12,
                            color:
                              v.status === 'error'
                                ? '#ef4444'
                                : v.status === 'warning'
                                ? '#f59e0b'
                                : '#22c55e',
                          }}
                        >
                          {v.year} {v.indicator}: 差异 {v.diffPercent.toFixed(2)}%
                        </div>
                      ))}
                    </div>
                  )}
                  <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 10 }}>
                    <button className="btn-text" onClick={handleExportCurrentData}>
                      ⬇️ 下载到本地
                    </button>
                  </div>
                </div>
              </Collapsible>
            )}

            {dataHistory.length > 0 && (
              <Collapsible title="📦 财务数据历史">
                <div className="data-history" style={{ marginTop: 0 }}>
                  <div className="data-history-title">最近3批</div>
                  {dataHistory.map((meta, idx) => (
                    <div key={idx} className="data-history-row">
                      <span className="data-history-time">{formatTimestamp(meta.timestamp)}</span>
                      <span className="data-history-source">{meta.sourceName}</span>
                      <span className="data-history-years">{meta.years?.length ? `${meta.years.length}年` : '-'}</span>
                      <button
                        className="btn-text"
                        style={{ marginLeft: 'auto', fontSize: 11 }}
                        onClick={() => handleExportHistoryData(meta.timestamp)}
                        title="下载该批次到本地"
                      >
                        ⬇️ 下载
                      </button>
                    </div>
                  ))}
                </div>
              </Collapsible>
            )}

            <Collapsible title="🚀 概念 & 风口">
              <div className="concept-panel" style={{ marginTop: 0, marginBottom: 0 }}>
                <div className="concept-wind">{concepts?.wind || '--'}</div>
                {concepts && concepts.concepts.length > 0 ? (
                  <div className="concept-tags">
                    {concepts.concepts.map((c, idx) => (
                      <span key={idx} className="concept-tag">{c}</span>
                    ))}
                  </div>
                ) : (
                  <div style={{ color: '#64748b', fontSize: 12, padding: '4px 0' }}>暂无概念数据</div>
                )}
              </div>
            </Collapsible>

            <Collapsible title="🏢 可比公司">
              <div className="comparable-panel" style={{ marginTop: 0, marginBottom: 0 }}>
                <div className="cp-search">
                  <input
                    type="text"
                    placeholder="添加可比公司 (3~5家)..."
                    value={compQuery}
                    disabled={loading || comparables.length >= 5}
                    onChange={(e) => {
                      setCompQuery(e.target.value)
                      setShowCompDropdown(true)
                    }}
                    onFocus={() => setShowCompDropdown(true)}
                  />
                  {showCompDropdown && compSuggestions.length > 0 && (
                    <ul className="dropdown cp-dropdown">
                      {compSuggestions.map((s) => (
                        <li
                          key={s.code}
                          onClick={() => handleAddComparable(s)}
                          className="dropdown-item"
                        >
                          <span className="stock-code">{s.code}</span>
                          <span className="stock-name">{s.name}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
                {comparables.length > 0 && (
                  <div className="cp-list">
                    {comparables.map((c) => {
                      const info = STOCKS.find((s) => s.code === c)
                      return (
                        <div key={c} className="cp-item">
                          <span className="cp-name">{info?.name || c}</span>
                          <button
                            className="cp-remove"
                            onClick={() => handleRemoveComparable(c)}
                            title="移除"
                          >
                            ×
                          </button>
                        </div>
                      )
                    })}
                  </div>
                )}
                <div className="cp-actions">
                  <button
                    className="btn-text cp-download"
                    onClick={handleDownloadComparables}
                    disabled={compDownloading || comparables.length === 0}
                  >
                    {compDownloading ? '下载中...' : '下载可比公司财报'}
                  </button>
                  {(() => {
                    const compChanged = JSON.stringify([...appliedComparables].sort()) !== JSON.stringify([...comparables].sort())
                    return (
                      <button
                        className={`btn-icon cp-merge${compChanged ? ' changed' : ''}`}
                        title={compChanged ? '可比公司已变更，点击下载最新财报并更新到报告' : '下载最新可比公司财报并更新到报告'}
                        onClick={handleAnalyzeWithComparables}
                        disabled={analyzing || comparables.length === 0}
                      >
                        {analyzing ? (
                          '···'
                        ) : (
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2.3"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            style={{ display: 'block' }}
                          >
                            <rect x="3" y="6" width="14" height="12" rx="2" ry="2" />
                            <path d="M17 12l4 4-4 4" />
                            <path d="M8 6V4a2 2 0 0 1 2-2h2" />
                            <polyline points="11 3 13 1 15 3" />
                          </svg>
                        )}
                      </button>
                    )
                  })()}
                </div>
              </div>
            </Collapsible>

            {currentSnapshot && (
              <Collapsible title="💡 亮点与风险">
                <div className="highlights-risks" style={{ marginTop: 0, marginBottom: 0 }}>
                  {highlights.length > 0 && (
                    <div className="hr-section">
                      <div className="hr-title">✅ 亮点</div>
                      {highlights.map((h, idx) => (
                        <div key={`h-${idx}`} className="highlight-item">
                          {h}
                        </div>
                      ))}
                    </div>
                  )}
                  {risks.length > 0 && (
                    <div className="hr-section">
                      <div className="hr-title">⚠️ 风险</div>
                      {risks.map((r, idx) => (
                        <div key={`r-${idx}`} className="risk-item">
                          {r}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </Collapsible>
            )}

            {quote && (
              <Collapsible title="📈 实时行情">
                <div className="stock-quote" style={{ marginTop: 0, marginBottom: 0 }}>
                  <div className="sq-header">
                    <div>
                      <span className={`sq-price ${quote.changePercent >= 0 ? 'up' : 'down'}`}>
                        {quote.currentPrice.toFixed(2)}
                      </span>
                      <span className={`sq-change ${quote.changePercent >= 0 ? 'up' : 'down'}`}>
                        {quote.changePercent >= 0 ? '+' : ''}
                        {quote.changePercent.toFixed(2)}% ({quote.changeAmount >= 0 ? '+' : ''}
                        {quote.changeAmount.toFixed(2)})
                      </span>
                    </div>
                    <div className="sq-time">
                      {quote.quoteTime || ''}
                    </div>
                  </div>
                  <div className="sq-grid">
                    <div className="sq-item">
                      <span className="sq-label">今开</span>
                      <span className="sq-value">{quote.open ? quote.open.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">最高</span>
                      <span className="sq-value">{quote.high ? quote.high.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">最低</span>
                      <span className="sq-value">{quote.low ? quote.low.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">昨收</span>
                      <span className="sq-value">{quote.previousClose ? quote.previousClose.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">换手率</span>
                      <span className="sq-value">{quote.turnoverRate ? `${quote.turnoverRate.toFixed(2)}%` : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">成交量</span>
                      <span className="sq-value">{formatAmount(quote.volume, '手')}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">成交额</span>
                      <span className="sq-value">{formatAmount(quote.turnoverAmount, '元')}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">振幅</span>
                      <span className="sq-value">{quote.amplitude ? `${quote.amplitude.toFixed(2)}%` : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">量比</span>
                      <span className="sq-value">{quote.volumeRatio ? quote.volumeRatio.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">流通市值</span>
                      <span className="sq-value">
                        {quote.circulatingMarketCap ? `${(quote.circulatingMarketCap / 100000000).toFixed(2)} 亿` : '-'}
                      </span>
                    </div>
                  </div>
                </div>
              </Collapsible>
            )}
            {quoteError && (
              <div className="quote-error">{quoteError}</div>
            )}

            {selectedStock && (
              <Collapsible title={`📈 K线走势${klines.length > 0 ? ` (${klines.length}日)` : ''}`}>
                <div className="stock-kline" style={{ marginTop: 0, marginBottom: 0 }}>
                  {klines.length > 0 ? (
                    <KlineChart data={klines} height={220} />
                  ) : klineError ? (
                    <div className="quote-error" style={{ marginTop: 4 }}>{klineError}</div>
                  ) : (
                    <div style={{ color: '#64748b', fontSize: 13, padding: '8px 0' }}>暂无K线数据（网络受限或该股票无数据）</div>
                  )}
                </div>
              </Collapsible>
            )}
          </>
        ) : (
          <div className="placeholder">
            <p>请从左侧自选列表中选择一只股票</p>
          </div>
        )}
      </section>

      {/* 右栏：报告展示 */}
      <section className="report-panel">
        <div className="report-tabs">
          <div className="report-tabs-left">
            {historyFiles.length > 0 && (
              <select
                className="history-select"
                value={viewingHistory || '__latest__'}
                onChange={handleHistoryChange}
              >
                <option value="__latest__">最新报告</option>
                {historyFiles.map((f) => (
                  <option key={f} value={f}>
                    {formatFilename(f)}
                  </option>
                ))}
              </select>
            )}
            {displayContent && (
              <select
                className="toc-select"
                value=""
                onChange={(e) => {
                  const id = e.target.value
                  if (id) {
                    handleTocJump(id)
                    const select = e.target
                    const label = tocSections.find((s) => s.id === id)?.label || '📑 跳转章节'
                    // 临时改写第一个 option 文本来模拟显示选中项
                    const firstOpt = select.querySelector('option:first-child') as HTMLOptionElement | null
                    if (firstOpt) {
                      firstOpt.textContent = '⬅ ' + label
                      firstOpt.value = ''
                    }
                    select.value = ''
                  }
                }}
              >
                <option value="" disabled>📑 跳转章节</option>
                {tocSections.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.label}
                  </option>
                ))}
              </select>
            )}

          </div>
          <div className="report-tabs-right">
            <div className="report-search-wrap">
              <input
                ref={reportSearchRef}
                type="text"
                className="report-search-input"
                placeholder="搜索报告内容"
                onKeyDown={handleReportSearchKeyDown}
                disabled={!displayContent}
                title={!displayContent ? '请先执行分析' : '输入关键词，按回车依次跳转匹配项'}
              />
            </div>
            <button
              className="btn-delete-report"
              onClick={handleDeleteReport}
              disabled={!displayContent}
              title={!displayContent ? '没有可删除的报告' : '删除当前显示的报告'}
            >
              删除报告
            </button>
            <button
              className="btn-download"
              onClick={handleReportDownload}
              disabled={!displayContent}
              title={!displayContent ? '请先执行分析' : '下载当前显示的报告'}
            >
              下载报告
            </button>
          </div>
        </div>
        <div className="report-content" ref={reportContentRef}>
          {displayContent ? (
            <div className="markdown-body" onClick={(e) => {
              const target = e.target as HTMLElement
              if (target.closest('.rim-adjust-btn')) {
                e.preventDefault()
                openRIMModal()
              }
            }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSlug, rehypeRaw]} components={markdownComponents}>
                {displayContent}
              </ReactMarkdown>
            </div>
          ) : selectedStock ? (
            <div className="placeholder">
              <p>【Markdown 报告展示区】</p>
              <p>选择股票后点击"18步分析"，报告将在此渲染</p>
            </div>
          ) : (
            <div className="placeholder">
              <p>未选择股票</p>
            </div>
          )}
        </div>
      </section>

      {/* 点击空白处关闭下拉 */}
      {showDropdown && (
        <div
          className="overlay"
          onClick={() => {
            setShowDropdown(false)
            inputRef.current?.blur()
          }}
        />
      )}

      {/* 计算溯源抽屉 */}
      {traceDrawerOpen && currentTrace && (
        <div className="trace-overlay" onClick={() => setTraceDrawerOpen(false)}>
          <div className="trace-drawer" onClick={(e) => e.stopPropagation()}>
            <div className="trace-header">
              <h3>
                {currentTrace.indicator}（{currentTrace.year}）计算过程
              </h3>
              <button className="trace-close" onClick={() => setTraceDrawerOpen(false)}>
                ×
              </button>
            </div>
            {traceList.length > 1 && (
              <div className="trace-switcher">
                <select
                  value={traceList.indexOf(currentTrace)}
                  onChange={(e) => {
                    const idx = Number(e.target.value)
                    if (traceList[idx]) setCurrentTrace(traceList[idx])
                  }}
                >
                  {(() => {
                    const groups: Record<string, analyzer.CalcTrace[]> = {}
                    traceList.forEach((t) => {
                      if (!groups[t.indicator]) groups[t.indicator] = []
                      groups[t.indicator].push(t)
                    })
                    Object.keys(groups).forEach((indicator) => {
                      groups[indicator].sort((a, b) => b.year.localeCompare(a.year))
                    })
                    return Object.entries(groups).map(([indicator, traces]) => (
                      <optgroup key={indicator} label={indicator}>
                        {traces.map((t) => {
                          const idx = traceList.indexOf(t)
                          return (
                            <option key={idx} value={idx}>
                              {t.year}
                            </option>
                          )
                        })}
                      </optgroup>
                    ))
                  })()}
                </select>
                <span className="trace-count">共 {traceList.length} 个指标</span>
              </div>
            )}
            <div className="trace-body">
              <div className="trace-section">
                <div className="trace-section-title">公式</div>
                <div className="trace-formula">{currentTrace.formula}</div>
              </div>
              <div className="trace-section">
                <div className="trace-section-title">原始数据</div>
                {currentTrace.inputs &&
                  Object.entries(currentTrace.inputs).map(([k, v]) => (
                    <div key={k} className="trace-input-row">
                      <span className="trace-input-name">
                        • {v.item}（{v.source}，{v.year}）
                      </span>
                      <span className="trace-input-value">{formatTraceValue(v.value)}</span>
                      {v.note && <span className="trace-input-note">{v.note}</span>}
                    </div>
                  ))}
              </div>
              <div className="trace-section">
                <div className="trace-section-title">计算步骤</div>
                {currentTrace.steps?.map((s, idx) => (
                  <div key={idx} className="trace-step">
                    <div className="trace-step-desc">
                      {idx + 1}. {s.desc}
                    </div>
                    <div className="trace-step-expr">{s.expr}</div>
                    <div className="trace-step-result">
                      = {formatTraceResult(s.value, currentTrace.indicator)}
                    </div>
                  </div>
                ))}
              </div>
              {currentTrace.note && (
                <div className="trace-section">
                  <div className="trace-section-title">💡 口径说明</div>
                  <div className="trace-note">{currentTrace.note}</div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* 强制重新分析弹窗 */}
      {forceAnalyzeOpen && (
        <div className="modal-overlay" onClick={() => setForceAnalyzeOpen(false)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <h4>数据未发生变化</h4>
            <p>
              上次分析时间：{lastAnalysisAt || '未知'}
              <br />
              当前财务数据与上次分析时一致，是否强制重新生成报告？
            </p>
            <div className="modal-actions">
              <button className="btn" onClick={() => setForceAnalyzeOpen(false)}>
                取消
              </button>
              <button
                className="btn primary"
                onClick={async () => {
                  setForceAnalyzeOpen(false)
                  await runAnalyze(true)
                }}
              >
                强制重新分析
              </button>
            </div>
          </div>
        </div>
      )}

      {/* RIM 参数调整弹窗 */}
      {showRIMModal && (
        <div className="modal-overlay" onClick={() => setShowRIMModal(false)}>
          <div className="modal-content rim-modal" onClick={(e) => e.stopPropagation()}>
            <h4>调整 RIM 估值参数</h4>
            <div className="rim-form">
              <div className="rim-row">
                <label>Beta</label>
                <input type="number" step={0.01} value={rimBeta} onChange={(e) => setRimBeta(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>无风险利率 Rf (%)</label>
                <input type="number" step={0.01} value={rimRf} onChange={(e) => setRimRf(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>市场风险溢价 Rm-Rf (%)</label>
                <input type="number" step={0.01} value={rimRmRf} onChange={(e) => setRimRmRf(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>永续增长率 g (%)</label>
                <input type="number" step={0.1} value={rimG} onChange={(e) => setRimG(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>每股净资产 BPS0</label>
                <input type="number" step={0.01} value={rimBPS0} onChange={(e) => setRimBPS0(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>当前股价</label>
                <input type="number" step={0.01} value={rimPrice} onChange={(e) => setRimPrice(Number(e.target.value))} />
              </div>
              <div className="rim-eps-title">预测期 EPS（至少填前3年）</div>
              <div className="rim-eps-grid">
                {rimEPS.map((v, i) => (
                  <div className="rim-eps-item" key={i}>
                    <label>第{i + 1}年</label>
                    <input type="text" inputMode="decimal" value={v} onChange={(e) => {
                      const val = e.target.value
                      if (!/^\d*\.?\d*$/.test(val)) return
                      const next = [...rimEPS]
                      next[i] = val === '' ? '0' : val
                      setRimEPS(next)
                    }} />
                  </div>
                ))}
              </div>
            </div>
            <div className="modal-actions">
              <button className="btn" onClick={() => setShowRIMModal(false)} disabled={rimLoading}>
                取消
              </button>
              <button className="btn primary" onClick={handleAnalyzeWithRIM} disabled={rimLoading || rimBPS0 <= 0 || rimEPS.map(Number).filter((v) => v > 0).length < 1}>
                {rimLoading ? (
                  <>
                    <span className="btn-progress" style={{ width: `${rimProgress}%` }} />
                    <span style={{ position: 'relative', zIndex: 1 }}>分析中 {rimProgress}%</span>
                  </>
                ) : '应用并重新分析'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default App
