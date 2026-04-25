import { useState, useEffect, useMemo, useRef, useCallback, Children, cloneElement } from 'react'
import './App.css'
import { STOCKS } from './stocks'
import { UnifiedChart } from './UnifiedChart'
import { FinancialTrendDrawer } from './FinancialTrendDrawer'
import { Settings, loadSettings, AppSettings } from './Settings'
import { ModuleCopyButton, setGlobalMarkdownContent } from './ModuleCopyButton'
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

function DetailsComponent({ children, ...props }: any) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDetailsElement>(null)

  useEffect(() => {
    if (!open) return
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [open])

  const wrappedChildren = Children.map(children, (child: any) => {
    if (!child) return child
    if (child.type === 'summary') {
      return cloneElement(child, {
        onClick: (e: React.MouseEvent) => {
          e.preventDefault()
          setOpen((prev) => !prev)
        }
      })
    }
    return child
  })

  return (
    <details ref={ref} open={open} {...props}>
      {wrappedChildren}
    </details>
  )
}

function InlineTooltip({ title, body }: { title: string; body: string }) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [open])

  return (
    <div ref={ref} className="inline-tooltip">
      <span className="inline-tooltip-trigger" onClick={() => setOpen((prev) => !prev)}>ℹ️</span>
      {open && (
        <div className="inline-tooltip-body">
          <strong>{title}</strong><br/>
          {body}
        </div>
      )}
    </div>
  )
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
import { toPng } from 'html-to-image'
import html2pdf from 'html2pdf.js'
import {
  GetWatchlist,
  GetWatchlistActivity,
  GetWatchlistFilterData,
  AddToWatchlist,
  RemoveFromWatchlist,
  ReorderWatchlist,
  ImportFinancialReports,
  DownloadReports,
  AnalyzeStock,
  AnalyzeStockWithRIM,
  CheckAnalysisCache,
  DownloadReport,
  ExportReportPDF,
  ExportReportImage,
  DeleteReport,
  ConfirmDialog,
  GetReport,
  GetStockDataHistory,
  GetStockProfile,
  RefreshStockProfile,
  GetComparables,
  AddComparable,
  RemoveComparable,
  DownloadComparableReports,
  FetchMissingActivity,
  GetStockQuote,
  // GetStockKlines,
  GetStockConcepts,
  ExportCurrentFinancialData,
  GetRiskRadar,
  UpdatePolicyLibrary,
  UpdateIndustryDatabase,
  GetIndustryDBMeta,
  GetIndustryTaskStatus,
  UpdateModule4Only,
  LoadAnalysisSnapshot,
  SendNotification,
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
type WatchlistFilterItem = main.WatchlistFilterItem
type RiskRadarItem = analyzer.RiskRadarItem
// type KlineData = downloader.KlineData

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
  // A-Score 为 0-100 分，越高风险越大，<60 为安全
  const ascore = getStepValue(steps, 8, latest, 'AScore')
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

  // A-Score 综合风险评分（A股适配）
  if (ascore < 40) highlights.push('A-Score 安全，财务质量良好')
  else if (ascore < 60) highlights.push('A-Score 低风险，财务质量可控')
  else if (ascore < 70) risks.push('A-Score 中风险，需关注财务健康度')
  else risks.push('A-Score 高风险，建议谨慎')

  if (growth >= 10) highlights.push('营收稳健增长')
  else if (growth < 0) risks.push('营收负增长，成长性承压')

  if (pg >= 10) highlights.push('净利润持续增长')
  else if (pg < 0) risks.push('净利润下滑，盈利能力减弱')

  if (cr >= 100) highlights.push('经营现金流充沛，盈利质量高')
  else if (cr > 0) risks.push('现金流含金量不足')

  return { highlights, risks }
}

function App() {
  const [watchlist, setWatchlist] = useState<WatchlistItem[]>([])
  const [selectedCode, setSelectedCode] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [showDropdown, setShowDropdown] = useState(false)
  const [loading, setLoading] = useState(false)
  const [settings, setSettings] = useState<AppSettings>(() => loadSettings())
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [downloadResult, setDownloadResult] = useState<DownloadResult | null>(null)
  const [downloading, setDownloading] = useState(false)
  const [downloadStatus, setDownloadStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [report, setReport] = useState<AnalysisReport | null>(null)
  const [snapshots, setSnapshots] = useState<Record<string, AnalysisReport>>({})
  const [analyzing, setAnalyzing] = useState(false)
  const [analyzeProgress, setAnalyzeProgress] = useState(0)
  const [viewingHistory, setViewingHistory] = useState<string | null>(null)
  const [historyContent, setHistoryContent] = useState<string>('')
  const [dataHistory, setDataHistory] = useState<HistoryMeta[]>([])
  const [profile, setProfile] = useState<StockProfile | null>(null)
  const [comparables, setComparables] = useState<string[]>([])
  const [appliedComparables, setAppliedComparables] = useState<string[]>([])
  const [compQuery, setCompQuery] = useState('')
  const [showCompDropdown, setShowCompDropdown] = useState(false)
  const [compDownloading, setCompDownloading] = useState(false)
  const [compReportsDownloaded, setCompReportsDownloaded] = useState(false)
  const [compDownloadStatus, setCompDownloadStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [fetchingActivity, setFetchingActivity] = useState(false)
  const [fetchActivityStatus, setFetchActivityStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})

  const [concepts, setConcepts] = useState<downloader.StockConcepts | null>(null)
  const [policyLibMeta, setPolicyLibMeta] = useState<{version: string, updatedAt: string} | null>(null)
  const [policyUpdating, setPolicyUpdating] = useState(false)
  const [industryDBMeta, setIndustryDBMeta] = useState<{version: string, updatedAt: string, count: number} | null>(null)
  const [industryUpdating, setIndustryUpdating] = useState(false)
  const [industryTask, setIndustryTask] = useState<any>(null)
  const [policyActionStatus, setPolicyActionStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [industryActionStatus, setIndustryActionStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [quote, setQuote] = useState<StockQuote | null>(null)
  const [quoteError, setQuoteError] = useState<string>('')
  // K线数据由 UnifiedChart 组件内部管理
  // const [klines, setKlines] = useState<KlineData[]>([])
  // const [klineError, setKlineError] = useState<string>('')
  const [activityMap, setActivityMap] = useState<Record<string, main.WatchlistActivitySummary>>({})
  const [activitySort, setActivitySort] = useState<'none' | 'desc' | 'asc'>('none')
  const [flashCode, setFlashCode] = useState<string | null>(null)
  const [filterData, setFilterData] = useState<Record<string, WatchlistFilterItem>>({})
  const [watchlistFilter, setWatchlistFilter] = useState<
    'none' | 'highReturn' | 'lowRisk' | 'hasData' | 'noData' | 'analyzed' | 'unanalyzed'
  >('none')
  const [watchlistIndustryFilter, setWatchlistIndustryFilter] = useState<string>('全部')
  const flashTimeoutRef = useRef<number | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const reportContentRef = useRef<HTMLDivElement>(null)
  const dragIndexRef = useRef<number | null>(null)
  const reportSearchRef = useRef<HTMLInputElement>(null)
  const reportMatchesRef = useRef<HTMLElement[]>([])
  const reportSearchIndexRef = useRef(0)
  const reportLastQueryRef = useRef('')
  const downloadMenuRef = useRef<HTMLDivElement>(null)
  const downloadMenuBtnRef = useRef<HTMLButtonElement>(null)
  const tocSelectRef = useRef<HTMLSelectElement>(null)
  const [traceDrawerOpen, setTraceDrawerOpen] = useState(false)
  const [currentTrace, setCurrentTrace] = useState<analyzer.CalcTrace | null>(null)
  const [traceList, setTraceList] = useState<analyzer.CalcTrace[]>([])
  const [forceAnalyzeOpen, setForceAnalyzeOpen] = useState(false)
  const [lastAnalysisAt, setLastAnalysisAt] = useState('')
  const [trendDrawerCode, setTrendDrawerCode] = useState<string | null>(null)
  const [riskRadar, setRiskRadar] = useState<RiskRadarItem[] | null>(null)
  const [downloadMenuOpen, setDownloadMenuOpen] = useState(false)

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
    { label: '模块11: 智能选股7大条件', id: '模块11-智能选股7大条件' },
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
  const [sidebarWidth, setSidebarWidth] = useState(230)
  const [isResizing, setIsResizing] = useState(false)

  useEffect(() => {
    if (!isResizing) return
    const handleMouseMove = (e: MouseEvent) => {
      const newWidth = Math.min(Math.max(e.clientX, 200), 400)
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
    GetWatchlistFilterData().then((list) => {
      const map: Record<string, WatchlistFilterItem> = {}
      ;(list || []).forEach((item) => {
        map[item.code] = item
      })
      setFilterData(map)
    }).catch((err) => {
      console.error('[GetWatchlistFilterData] error', err)
    })
    // 加载政策库元信息
    loadPolicyLibMeta()
    // 加载行业数据库元信息，并根据设置决定是否自动更新
    const autoUpdateIndustry = async () => {
      const meta = await GetIndustryDBMeta()
      const formatted = {
        version: meta.version || '1.0',
        updatedAt: meta.updatedAt || '未更新',
        count: meta.count || 0,
      }
      setIndustryDBMeta(formatted)
      if (!settings.autoUpdateIndustryDB) return
      if (formatted.updatedAt === '未更新') {
        handleUpdateIndustryDB()
        return
      }
      try {
        const last = new Date(formatted.updatedAt.replace(/-/g, '/'))
        const days = (Date.now() - last.getTime()) / (1000 * 60 * 60 * 24)
        if (days >= 7) {
          handleUpdateIndustryDB()
        }
      } catch {
        // ignore
      }
    }
    autoUpdateIndustry()
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
    // 应用主题
    const effectiveTheme = settings.theme === 'system' 
      ? (window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark')
      : settings.theme
    
    if (effectiveTheme === 'light') {
      document.body.classList.add('light')
    } else {
      document.body.classList.remove('light')
    }
  }, [settings.theme])

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
    let list = [...watchlist]

    // 应用筛选条件
    if (watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部') {
      list = list.filter((s) => {
        const fd = filterData[s.code]
        if (!fd) return false

        if (watchlistIndustryFilter !== '全部' && fd.industry !== watchlistIndustryFilter) {
          return false
        }

        switch (watchlistFilter) {
          case 'highReturn':
            return fd.shareholderReturnRate > 0.10
          case 'lowRisk':
            return fd.aScore > 0 && fd.aScore < 60
          case 'hasData':
            return fd.hasFinancialData
          case 'noData':
            return !fd.hasFinancialData
          case 'analyzed':
            return fd.hasSnapshot
          case 'unanalyzed':
            return !fd.hasSnapshot
          default:
            return true
        }
      })
    }

    if (activitySort === 'none') return list
    list.sort((a, b) => {
      const scoreA = activityMap[a.code]?.score ?? -1
      const scoreB = activityMap[b.code]?.score ?? -1
      if (activitySort === 'desc') return scoreB - scoreA
      return scoreA - scoreB
    })
    return list
  }, [watchlist, activityMap, activitySort, filterData, watchlistFilter, watchlistIndustryFilter])

  // 通过 data-status 属性控制 activity-hint 状态显示（避免直接操作 DOM）
  useEffect(() => {
    if (!reportContentRef.current) return
    const trigger = reportContentRef.current.querySelector('.fetch-activity-trigger')
    if (!trigger) return
    if (fetchingActivity) {
      trigger.setAttribute('data-status', 'loading')
    } else if (fetchActivityStatus.type === 'success') {
      trigger.setAttribute('data-status', 'success')
    } else if (fetchActivityStatus.type === 'error') {
      trigger.setAttribute('data-status', 'error')
    } else {
      trigger.removeAttribute('data-status')
    }
  }, [fetchingActivity, fetchActivityStatus])

  // 当切换股票时，若内存中没有快照，尝试从磁盘加载
  useEffect(() => {
    if (!selectedStock) return
    if (snapshots[selectedStock.code]) return
    LoadAnalysisSnapshot(selectedStock.code)
      .then((snapshot) => {
        if (snapshot) {
          setSnapshots((prev) => ({ ...prev, [selectedStock.code]: snapshot }))
        }
      })
      .catch(() => {
        // 忽略加载失败的错误
      })
  }, [selectedStock, snapshots])

  const currentSnapshot = selectedStock ? snapshots[selectedStock.code] : null
  const { highlights, risks } = useMemo(() => {
    if (!currentSnapshot) return { highlights: [], risks: [] }
    // 优先使用后端统一生成的亮点/风险，fallback 到前端本地计算
    if (currentSnapshot.highlights && currentSnapshot.highlights.length > 0) {
      return { highlights: currentSnapshot.highlights, risks: currentSnapshot.risks || [] }
    }
    return extractHighlightsAndRisks(currentSnapshot)
  }, [currentSnapshot])

  const loadReportHistory = useCallback(async (code: string, autoLoadLatest = false) => {
    try {
      // 获取分析缓存时间
      const cache = await CheckAnalysisCache(code)
      setLastAnalysisAt(cache?.lastAnalysisAt || '')
      if (autoLoadLatest) {
        const content = await GetReport(code, 'latest.md')
        if (content) {
          setHistoryContent(content)
          setViewingHistory('latest.md')
        } else {
          setHistoryContent('')
          setViewingHistory(null)
        }
      }
    } catch {
      setLastAnalysisAt('')
    }
  }, [])

  const loadDataHistory = useCallback(async (code: string) => {
    try {
      console.log('[loadDataHistory] Loading for:', code)
      const list = await GetStockDataHistory(code)
      console.log('[loadDataHistory] Result:', list)
      setDataHistory(list || [])
    } catch (err: any) {
      console.error('[loadDataHistory] Error:', err)
      setDataHistory([])
    }
  }, [])

  const loadProfile = useCallback(async (code: string) => {
    try {
      const p = await GetStockProfile(code)
      setProfile(p || null)
      return p || null
    } catch {
      setProfile(null)
      return null
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

  // 加载政策库元信息（从 localStorage 或默认值）
  const loadPolicyLibMeta = useCallback(() => {
    try {
      const saved = localStorage.getItem('policy_library_meta')
      if (saved) {
        setPolicyLibMeta(JSON.parse(saved))
      } else {
        setPolicyLibMeta({ version: 'builtin', updatedAt: '内置默认' })
      }
    } catch {
      setPolicyLibMeta({ version: 'builtin', updatedAt: '内置默认' })
    }
  }, [])

  // 更新政策库
  const handleUpdatePolicyLibrary = useCallback(async () => {
    setPolicyUpdating(true)
    setPolicyActionStatus({type: null, message: ''})
    try {
      const result = await UpdatePolicyLibrary()
      if (result) {
        const meta = { version: result.path ? 'external' : 'builtin', updatedAt: new Date().toLocaleString('zh-CN') }
        setPolicyLibMeta(meta)
        localStorage.setItem('policy_library_meta', JSON.stringify(meta))
        const msg = `政策库更新成功：新增行业关键词 ${result.added_industry_keywords} 个，概念关键词 ${result.added_concept_keywords} 个，行业 ${result.total_industries} 个，概念 ${result.total_concepts} 个`
        setPolicyActionStatus({type: 'success', message: msg})
        setTimeout(() => setPolicyActionStatus({type: null, message: ''}), 3000)
      }
    } catch (err: any) {
      const msg = '政策库更新失败: ' + (err?.message || String(err))
      setPolicyActionStatus({type: 'error', message: msg.length > 100 ? msg.slice(0, 100) + '...' : msg})
      setTimeout(() => setPolicyActionStatus({type: null, message: ''}), 5000)
    } finally {
      setPolicyUpdating(false)
    }
  }, [])

  // 加载行业数据库元信息
  const loadIndustryDBMeta = useCallback(async () => {
    try {
      const meta = await GetIndustryDBMeta()
      setIndustryDBMeta({
        version: meta.version || '1.0',
        updatedAt: meta.updatedAt || '未更新',
        count: meta.count || 0
      })
    } catch {
      setIndustryDBMeta({ version: '1.0', updatedAt: '未更新', count: 0 })
    }
  }, [])

  // 轮询后台行业数据采集任务状态
  useEffect(() => {
    let prevStatus = ''
    const check = async () => {
      try {
        const task = await GetIndustryTaskStatus()
        setIndustryTask(task)
        const status = task?.status || 'idle'
        if (status === 'running') {
          setIndustryUpdating(true)
        } else {
          setIndustryUpdating(false)
        }
        // 如果刚从 running 变为 completed，刷新元信息
        if (prevStatus === 'running' && status === 'completed') {
          loadIndustryDBMeta()
        }
        prevStatus = status
      } catch {
        // ignore
      }
    }
    check()
    const id = setInterval(check, 3000)
    return () => clearInterval(id)
  }, [loadIndustryDBMeta])

  // 更新行业数据库
  const handleUpdateIndustryDB = useCallback(async () => {
    setIndustryUpdating(true)
    setIndustryActionStatus({type: null, message: ''})
    try {
      const result = await UpdateIndustryDatabase()
      if (result) {
        await loadIndustryDBMeta()
        const msg = `行业数据库更新成功：更新行业 ${result.updated_count} 个，跳过 ${result.skipped_count} 个，行业总数 ${result.total_industries} 个`
        setIndustryActionStatus({type: 'success', message: msg})
        setTimeout(() => setIndustryActionStatus({type: null, message: ''}), 5000)
      }
    } catch (err: any) {
      console.error('更新行业数据库失败:', err)
      const msg = '行业数据库更新失败: ' + (err?.message || String(err))
      setIndustryActionStatus({type: 'error', message: msg.length > 100 ? msg.slice(0, 100) + '...' : msg})
      setTimeout(() => setIndustryActionStatus({type: null, message: ''}), 5000)
    } finally {
      setIndustryUpdating(false)
    }
  }, [loadIndustryDBMeta])

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

  const loadRiskRadar = useCallback(async (code: string, industry: string) => {
    try {
      const items = await GetRiskRadar(code, industry)
      setRiskRadar(items || [])
    } catch (err: any) {
      setRiskRadar([])
      console.error('风险雷达加载失败:', err)
    }
  }, [])


  // K线数据由 UnifiedChart 组件内部管理
  // const loadKlines = useCallback(async (code: string) => {
  //   try {
  //     setKlineError('')
  //     const list = await GetStockKlines(code)
  //     setKlines(list || [])
  //   } catch (err: any) {
  //     setKlines([])
  //     setKlineError('K线数据获取失败')
  //     console.error('K线加载失败:', err)
  //   }
  // }, [])

  const handleSelectSuggestion = async (stock: Stock) => {
    setQuery('')
    setShowDropdown(false)
    setLoading(true)
    try {
      await AddToWatchlist(stock.code)
      const list = await GetWatchlist()
      setWatchlist(list || [])
      // 刷新筛选数据
      GetWatchlistFilterData().then((fd) => {
        const map: Record<string, WatchlistFilterItem> = {}
        ;(fd || []).forEach((item) => { map[item.code] = item })
        setFilterData(map)
      })
      setSelectedCode(stock.code)
      setProfile(null)
      setQuote(null)
      setQuoteError('')
      // setKlines([])
      // setKlineError('')
      setDownloadResult(null)
      setReport(null)
      setViewingHistory(null)
      setHistoryContent('')
      setCompReportsDownloaded(false)
      await loadReportHistory(stock.code, true)
      await loadDataHistory(stock.code)
      const p = await loadProfile(stock.code)
      await loadConcepts(stock.code)
      await loadComparables(stock.code)
      await loadQuote(stock.code)
      await loadRiskRadar(stock.code, p?.industry || '')
      // await loadKlines(stock.code)
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
      // 刷新筛选数据
      GetWatchlistFilterData().then((fd) => {
        const map: Record<string, WatchlistFilterItem> = {}
        ;(fd || []).forEach((item) => { map[item.code] = item })
        setFilterData(map)
      })
      if (selectedCode === code) {
        setSelectedCode(null)
        setProfile(null)
        setQuote(null)
        setQuoteError('')
        // setKlines([])
        // setKlineError('')
        setImportResult(null)
        setDownloadResult(null)
        setReport(null)
        setViewingHistory(null)
        setHistoryContent('')
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
    setDownloadStatus({type: null, message: ''})
    try {
      const maxYears = typeof settings.reportYears === 'number' && settings.reportYears > 0
        ? Math.floor(settings.reportYears)
        : 5
      const result = await Promise.race([
        DownloadReports(selectedStock.code, maxYears),
        new Promise<never>((_, reject) =>
          setTimeout(() => reject(new Error('下载超时，请检查网络或刷新页面后重试')), 30000)
        )
      ]) as Awaited<ReturnType<typeof DownloadReports>>
      setDownloadResult(result)
      if (result.success) {
        // 简化消息：年份多时显示范围
        const years = result.years || []
        let yearStr = ''
        if (years.length > 0) {
          if (years.length <= 3) {
            yearStr = years.join(', ')
          } else {
            yearStr = `${years[0]}～${years[years.length - 1]}`
          }
        }
        const msg = `✅ 下载成功${yearStr ? '，包含' + yearStr + '年' : ''}`
        setDownloadStatus({type: 'success', message: msg})
        console.log('[handleDownload] Reloading data history for:', selectedStock.code)
        await loadDataHistory(selectedStock.code)
        console.log('[handleDownload] Data history reloaded, count:', dataHistory.length)
        // 刷新筛选数据
        GetWatchlistFilterData().then((fd) => {
          const map: Record<string, WatchlistFilterItem> = {}
          ;(fd || []).forEach((item) => { map[item.code] = item })
          setFilterData(map)
        })
        // 3秒后清除成功消息
        setTimeout(() => setDownloadStatus({type: null, message: ''}), 3000)
      } else {
        setDownloadStatus({type: 'error', message: '❌ 下载失败'})
      }
    } catch (err: any) {
      console.error('下载失败:', err)
      const msg = err?.message || String(err)
      if (msg.includes('companyType') || msg.includes('未找到') || msg.includes('无数据') || msg.includes('无法确定')) {
        setDownloadStatus({type: 'error', message: '❌ 该股票财报暂不可从网络获取，建议手动导入CSV'})
      } else if (msg.includes('timeout') || msg.includes('Timeout') || msg.includes('超时')) {
        setDownloadStatus({type: 'error', message: '❌ 网络超时，请稍后重试'})
      } else {
        setDownloadStatus({type: 'error', message: '❌ ' + msg})
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

  const runAnalyze = async (overwriteLatest = false) => {
    if (!selectedStock) {
      alert('没有选择股票')
      return
    }
    
    setAnalyzing(true)
    setAnalyzeProgress(5)
    const interval = setInterval(() => {
      setAnalyzeProgress((p) => {
        if (p >= 85) return 90 // 停在90%，等待网络数据
        return p + 3
      })
    }, 400)
    try {
      const result = await AnalyzeStock(selectedStock.code, overwriteLatest)
      setReport(result)
      setViewingHistory(null)
      setHistoryContent('')
      if (settings.analysisNotification) {
        SendNotification('分析完成', `${selectedStock.name || selectedStock.code} 的财报分析已完成`).catch(() => {})
      }
      if (result) {
        setSnapshots((prev) => ({ ...prev, [selectedStock.code]: result }))
      }
      setAppliedComparables(comparables)
      await loadReportHistory(selectedStock.code)
      // 刷新筛选数据
      GetWatchlistFilterData().then((fd) => {
        const map: Record<string, WatchlistFilterItem> = {}
        ;(fd || []).forEach((item) => { map[item.code] = item })
        setFilterData(map)
      })
    } catch (err: any) {
      console.error('分析失败:', err)
      const errorMsg = err?.message || String(err) || '未知错误'
      alert('分析失败: ' + errorMsg)
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
    if (!selectedStock) {
      alert('请选择一只股票')
      return
    }
    
    // 检查是否有财务数据
    if (dataHistory.length === 0) {
      alert('请先下载或导入财报数据')
      return
    }
    
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
      // 继续执行分析，不要阻塞用户
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
      if (settings.analysisNotification) {
        SendNotification('分析完成', `${selectedStock.name || selectedStock.code} 的财报分析（含RIM估值）已完成`).catch(() => {})
      }
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
    if (!selectedStock || !displayContent) {
      return
    }
    const content = viewingHistory ? historyContent : report?.markdownContent
    if (!content) {
      alert('没有可下载的报告内容')
      return
    }
    try {
      await DownloadReport(selectedStock.code, content)
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) {
        return
      }
      alert('下载报告失败: ' + msg)
    }
  }

  const handleExportPDF = async () => {
    if (!selectedStock || !reportContentRef.current) return
    const markdownBody = reportContentRef.current.querySelector('.markdown-body') as HTMLElement | null
    if (!markdownBody) {
      alert('未找到报告内容')
      return
    }
    try {
      const opt: any = {
        margin: [10, 10, 10, 10],
        filename: `${selectedStock.code}_投资分析报告.pdf`,
        image: { type: 'jpeg', quality: 0.98 },
        html2canvas: { scale: 2, useCORS: true },
        jsPDF: { unit: 'mm', format: 'a4', orientation: 'portrait' },
      }
      const pdfDataUrl: string = await html2pdf().set(opt).from(markdownBody).outputPdf('datauristring')
      // 去掉 data:application/pdf;base64, 前缀
      const base64Data = pdfDataUrl.split(',')[1]
      await ExportReportPDF(selectedStock.code, base64Data)
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) {
        return
      }
      alert('导出PDF失败: ' + msg)
    }
  }

  const handleDownloadImage = async () => {
    if (!selectedStock || !reportContentRef.current) return
    const markdownBody = reportContentRef.current.querySelector('.markdown-body') as HTMLElement | null
    if (!markdownBody) {
      alert('没有可下载的报告内容')
      return
    }
    try {
      const dataUrl = await toPng(markdownBody, {
        quality: 0.95,
        backgroundColor: getComputedStyle(document.body).backgroundColor,
        pixelRatio: 2,
      })
      await ExportReportImage(selectedStock.code, dataUrl)
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) {
        return
      }
      alert('生成图片失败: ' + msg)
    }
  }

  // 下载菜单点击外部关闭
  useEffect(() => {
    if (!downloadMenuOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (
        downloadMenuRef.current &&
        !downloadMenuRef.current.contains(e.target as Node) &&
        downloadMenuBtnRef.current &&
        !downloadMenuBtnRef.current.contains(e.target as Node)
      ) {
        setDownloadMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [downloadMenuOpen])

  const handleDeleteReport = async () => {
    if (!selectedStock || !displayContent) {
      return
    }
    const filename = viewingHistory || 'latest.md'
    const confirmed = await ConfirmDialog('确认删除', `确定删除报告 ${filename} 吗？`)
    if (!confirmed) {
      return
    }
    try {
      await DeleteReport(selectedStock.code, filename)
      setViewingHistory(null)
      setHistoryContent('')
      setReport(null)
      setLastAnalysisAt('')
      // 同时清理该股票的快照，避免左下角亮点与风险面板仍显示旧数据
      setSnapshots((prev) => {
        const next = { ...prev }
        delete next[selectedStock.code]
        return next
      })
    } catch (err: any) {
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
      setCompReportsDownloaded(false) // 可比公司变化，需要重新下载
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
      setCompReportsDownloaded(false) // 可比公司变化，需要重新下载
    } catch (err: any) {
      alert(String(err))
    }
  }

  const handleDownloadComparables = async () => {
    if (!selectedStock || comparables.length === 0) return
    setCompDownloading(true)
    setCompDownloadStatus({type: null, message: ''})
    try {
      const result = await DownloadComparableReports(selectedStock.code)
      if (result) {
        if (result.success) {
          setCompReportsDownloaded(true)
          setCompDownloadStatus({type: 'success', message: result.message})
          setTimeout(() => setCompDownloadStatus({type: null, message: ''}), 3000)
        } else {
          setCompDownloadStatus({type: 'error', message: result.message || '下载失败'})
          setTimeout(() => setCompDownloadStatus({type: null, message: ''}), 5000)
        }
      }
    } catch (err: any) {
      console.error('下载可比公司财报失败:', err)
      const msg = err?.message || String(err)
      setCompDownloadStatus({type: 'error', message: msg.length > 60 ? msg.slice(0, 60) + '...' : msg})
      setTimeout(() => setCompDownloadStatus({type: null, message: ''}), 5000)
    } finally {
      setCompDownloading(false)
    }
  }

  const handleFetchMissingActivity = async () => {
    if (!selectedStock || comparables.length === 0) return
    setFetchingActivity(true)
    setFetchActivityStatus({type: null, message: ''})
    // 保存当前滚动位置，避免更新后跳动
    const scrollContainer = reportContentRef.current
    const scrollTop = scrollContainer?.scrollTop ?? 0
    try {
      const result = await FetchMissingActivity(comparables)
      if (result && result.successCount > 0) {
        setFetchActivityStatus({type: 'success', message: '正在更新模块4...'})
        const module4Result = await UpdateModule4Only(selectedStock.code)
        if (module4Result) {
          // 只更新 markdownContent，保留报告其他字段，避免右栏跳动
          if (report) {
            setReport({ ...report, markdownContent: module4Result.markdownContent } as AnalysisReport)
          } else {
            setReport(module4Result)
          }
          setSnapshots((prev) => ({ ...prev, [selectedStock.code]: module4Result }))
          setFetchActivityStatus({type: 'success', message: `已更新 ${result.successCount} 家公司活跃度`})
          // 恢复滚动位置
          requestAnimationFrame(() => {
            if (reportContentRef.current) {
              reportContentRef.current.scrollTop = scrollTop
            }
          })
        }
      } else if (result && result.failCount > 0) {
        setFetchActivityStatus({type: 'error', message: result.message || '获取失败'})
      } else {
        setFetchActivityStatus({type: 'success', message: '所有公司活跃度已是最新'})
      }
    } catch (err: any) {
      console.error('获取缺失活跃度失败:', err)
      setFetchActivityStatus({type: 'error', message: err?.message || String(err)})
    } finally {
      setFetchingActivity(false)
      setTimeout(() => setFetchActivityStatus({type: null, message: ''}), 4000)
    }
  }

  const handleAnalyzeWithComparables = async () => {
    if (!selectedStock || comparables.length === 0) return
    setAnalyzing(true)
    setAnalyzeProgress(5)
    const interval = setInterval(() => {
      setAnalyzeProgress((p) => (p >= 90 ? 90 : p + 5))
    }, 300)
    try {
      // 只更新模块4，不重新下载财报，不跑完整分析
      const result = await UpdateModule4Only(selectedStock.code)
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
      console.error('更新模块4失败:', err)
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

  // 切换报告时清除搜索高亮和 trace，并更新全局 Markdown 内容
  useEffect(() => {
    clearSearchHighlights()
    reportLastQueryRef.current = ''
    if (reportSearchRef.current) {
      reportSearchRef.current.value = ''
    }
    setTraceDrawerOpen(false)
    setCurrentTrace(null)
    setTraceList([])
    // 设置全局 Markdown 内容供模块复制功能使用
    setGlobalMarkdownContent(displayContent || '')
  }, [displayContent])

  // 报告内容滚动时联动更新"跳转章节"下拉框显示
  useEffect(() => {
    const container = reportContentRef.current
    if (!container || !displayContent) return

    let rafId: number | null = null
    const handleScroll = () => {
      if (rafId) return
      rafId = requestAnimationFrame(() => {
        rafId = null
        const headings = container.querySelectorAll('h1')
        if (headings.length === 0 || !tocSelectRef.current) return
        const containerTop = container.getBoundingClientRect().top
        let closest: Element | null = null
        let closestOffset = Infinity
        for (const h of headings) {
          const offset = h.getBoundingClientRect().top - containerTop
          if (offset >= -40 && offset < closestOffset) {
            closest = h
            closestOffset = offset
          }
        }
        // 如果所有标题都在上方，取最后一个
        if (!closest && headings.length > 0) {
          closest = headings[headings.length - 1]
        }
        if (closest) {
          const id = closest.id
          const label = tocSections.find((s) => s.id === id)?.label || '📑 跳转章节'
          const firstOpt = tocSelectRef.current.querySelector('option:first-child') as HTMLOptionElement | null
          if (firstOpt) {
            firstOpt.textContent = '⬅ ' + label
          }
        }
      })
    }
    container.addEventListener('scroll', handleScroll)
    // 初始触发一次
    handleScroll()
    return () => {
      container.removeEventListener('scroll', handleScroll)
      if (rafId) cancelAnimationFrame(rafId)
    }
  }, [displayContent])

  const markdownComponents = useMemo(() => ({
    details: DetailsComponent,
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
              const sourceReport = report || currentSnapshot
              const matched =
                sourceReport?.stepResults?.flatMap((step) =>
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
    div({ className, children, ...props }: any) {
      const code = selectedStock?.code || ''
      if (className === 'chart-unified' && code) {
        return <UnifiedChart code={code} quote={quote || undefined} />
      }
      return (
        <div className={className} {...props}>
          {children}
        </div>
      )
    },
    // 为模块标题添加复制按钮（仅 h1 级别的模块标题）
    h1({ children, id, ...props }: any) {
      const titleText = children?.toString() || ''
      // 匹配模块标题：模块X: 标题
      const isModuleTitle = /^模块\d+/.test(titleText)
      const isModule8 = titleText.includes('模块8')
      // 强制修正模块8的 id，确保 TOC 导航匹配
      const headingId = isModule8 ? '模块8-a-score-综合风险画像' : id
      // 过滤掉 children 中的 trace-trigger（旧版后端可能残留）
      const filteredChildren = isModule8
        ? Children.map(children, (child: any) => {
            if (child && typeof child === 'object' && typeof child.props?.className === 'string' && child.props.className.includes('trace-trigger')) {
              return null
            }
            return child
          })
        : children
      
      return (
        <h1 id={headingId} {...props} style={{ position: 'relative', display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingRight: isModule8 ? '52px' : '32px' }}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {filteredChildren}
            {isModule8 && (
              <InlineTooltip
                title="A-Score 综合风险画像"
                body="A-Score（0-100分）综合评估企业财务风险，分数越高，潜在隐患越大。基于公开财务报表与监管信息，从六个维度打分：财务造假风险、偿债能力、现金流质量、应收账款健康度、盈利稳定性，以及股权质押/减持/监管问询等非财务信号。其中财务维度适用于 A 股与港股，非财务信号目前主要覆盖 A 股。评判标准：< 40分安全，40-60分低风险，60-70分中风险（需深入核查），≥ 70分高危（建议回避）。"
              />
            )}
          </span>
          {isModuleTitle && (
            <ModuleCopyButton moduleId={headingId || ''} moduleTitle={titleText} />
          )}
        </h1>
      )
    },
  }), [report, selectedStock])

  return (
    <div className="app">
      {/* 设置按钮 */}
      <Settings 
        settings={settings} 
        onSettingsChange={setSettings}
        policyLibMeta={policyLibMeta}
        industryDBMeta={industryDBMeta}
        policyUpdating={policyUpdating}
        industryUpdating={industryUpdating}
        onUpdatePolicyLibrary={handleUpdatePolicyLibrary}
        onUpdateIndustryDB={handleUpdateIndustryDB}
        policyActionStatus={policyActionStatus}
        industryActionStatus={industryActionStatus}
        industryTask={industryTask}
      />

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

        {(() => {
          const industries = Array.from(
            new Set(Object.values(filterData).map((d) => d.industry).filter(Boolean))
          ).sort()
          const filterButtons: { key: typeof watchlistFilter; label: string }[] = [
            { key: 'none', label: '全部' },
            { key: 'highReturn', label: '高回报' },
            { key: 'lowRisk', label: '低风险' },
            { key: 'hasData', label: '有财报' },
            { key: 'noData', label: '未下载' },
            { key: 'analyzed', label: '已分析' },
            { key: 'unanalyzed', label: '未分析' },
          ]
          const activeFilterLabel = filterButtons.find((b) => b.key === watchlistFilter)?.label
          const hasFilter = watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部'
          let title = '🔍 筛选器'
          if (hasFilter) {
            const parts: string[] = []
            if (watchlistFilter !== 'none') parts.push(activeFilterLabel!)
            if (watchlistIndustryFilter !== '全部') parts.push(watchlistIndustryFilter)
            title += ` · ${parts.join(' · ')} (${displayWatchlist.length}/${watchlist.length}只)`
          }
          return (
            <Collapsible title={title} defaultExpanded={false}>
              <div className="watchlist-filters" style={{ padding: '8px 0 4px' }}>
                <div style={{ display: 'flex', gap: '3px', flexWrap: 'wrap', alignItems: 'center', marginBottom: '6px' }}>
                  {filterButtons.map((btn) => (
                    <button
                      key={btn.key}
                      onClick={() => setWatchlistFilter(btn.key)}
                      style={{
                        padding: '2px 5px',
                        fontSize: '11px',
                        borderRadius: '4px',
                        border: '1px solid ' + (watchlistFilter === btn.key ? '#3b82f6' : 'rgba(148,163,184,0.3)'),
                        background: watchlistFilter === btn.key ? '#3b82f6' : 'transparent',
                        color: watchlistFilter === btn.key ? '#fff' : '#94a3b8',
                        cursor: 'pointer',
                        lineHeight: 1.4,
                      }}
                    >
                      {btn.label}
                    </button>
                  ))}
                  {industries.length > 0 && (
                    <select
                      value={watchlistIndustryFilter}
                      onChange={(e) => setWatchlistIndustryFilter(e.target.value)}
                      style={{
                        padding: '3px 6px',
                        fontSize: '12px',
                        borderRadius: '4px',
                        border: '1px solid rgba(148,163,184,0.3)',
                        background: 'transparent',
                        color: '#94a3b8',
                        marginLeft: '4px',
                      }}
                    >
                      <option value="全部">全部行业</option>
                      {industries.map((ind) => (
                        <option key={ind} value={ind}>{ind}</option>
                      ))}
                    </select>
                  )}
                </div>
                <div style={{ fontSize: '11px', color: '#64748b' }}>
                  显示 {displayWatchlist.length} / {watchlist.length} 只
                  {hasFilter && (
                    <button
                      onClick={() => { setWatchlistFilter('none'); setWatchlistIndustryFilter('全部') }}
                      style={{
                        marginLeft: '8px',
                        fontSize: '11px',
                        color: '#3b82f6',
                        background: 'none',
                        border: 'none',
                        cursor: 'pointer',
                        padding: 0,
                      }}
                    >
                      清除筛选
                    </button>
                  )}
                </div>
              </div>
            </Collapsible>
          )
        })()}

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
            热度
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
                draggable={activitySort === 'none' && watchlistFilter === 'none' && watchlistIndustryFilter === '全部'}
                className={`${selectedCode === s.code ? 'active' : ''}${flashCode === s.code ? ' flash-match' : ''}`}
                onDragStart={() => {
                  dragIndexRef.current = idx
                }}
                onDragOver={(e) => {
                  if (activitySort !== 'none' || watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部') return
                  e.preventDefault()
                }}
                onDrop={(e) => {
                  if (activitySort !== 'none' || watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部') return
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
                  // setKlines([])
                  // setKlineError('')
                  setImportResult(null)
                  setDownloadResult(null)
                  setReport(null)
                  setViewingHistory(null)
                  setHistoryContent('')
                  setComparables([])
                  loadReportHistory(s.code, true)
                  loadDataHistory(s.code)
                  loadProfile(s.code).then((p) => loadRiskRadar(s.code, p?.industry || ''))
                  loadConcepts(s.code)
                  loadComparables(s.code)
                  loadQuote(s.code)
                  // loadKlines(s.code)
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
          {displayWatchlist.length === 0 && (
            <li className="watchlist-empty" style={{ padding: '24px 12px', textAlign: 'center', color: '#64748b', fontSize: '13px', listStyle: 'none' }}>
              {watchlist.length === 0 ? (
                <>
                  <div style={{ marginBottom: '8px', fontSize: '16px' }}>🔍</div>
                  <div>自选列表为空</div>
                  <div style={{ marginTop: '4px', fontSize: '12px', opacity: 0.8 }}>在上方搜索框输入代码或名称添加股票</div>
                </>
              ) : (
                <>
                  <div style={{ marginBottom: '8px', fontSize: '16px' }}>🍃</div>
                  <div>没有符合条件的股票</div>
                  <div style={{ marginTop: '4px', fontSize: '12px', opacity: 0.8 }}>尝试调整筛选条件</div>
                </>
              )}
            </li>
          )}
        </ul>

        <div className="watchlist-footer">
          {(watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部')
            ? `显示 ${displayWatchlist.length} 只（全部 ${watchlist.length} / 100）`
            : `共 ${watchlist.length} / 100 只`}
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
                <div className="stock-info-item">
                  <span className="stock-info-label">股东回报率</span>
                  {(() => {
                    const rate = quote?.shareholderReturnRate
                    if (rate == null || rate <= 0) {
                      return <span className="stock-info-value">--</span>
                    }
                    let color = '#94a3b8'
                    if (rate > 0.10) color = '#22c55e'
                    else if (rate >= 0.06) color = '#eab308'
                    const dy = quote?.dividendYield || 0
                    const ey = rate - dy
                    const tooltip = `股东回报率 ≈ 盈利收益率(ROE/PB) + 股息率\n当前: ${(ey * 100).toFixed(2)}% + ${(dy * 100).toFixed(2)}% = ${(rate * 100).toFixed(2)}%\n假设公司维持当前盈利能力且估值不变，该数字可视为股东每年的名义总回报。`
                    return (
                      <span
                        className="stock-info-value"
                        style={{ color, cursor: 'help' }}
                        title={tooltip}
                      >
                        {(rate * 100).toFixed(2)}%
                      </span>
                    )
                  })()}
                </div>
              </div>
              <div className="stock-info-footer">
                <span className="stock-info-time">
                  {profile?.updatedAt
                    ? `更新于: ${new Date(profile.updatedAt).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}`
                    : '暂无数据'}
                </span>
                <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                  <button
                    className="stock-info-refresh"
                    onClick={() => setTrendDrawerCode(selectedStock!.code)}
                    title="查看近5年财务指标趋势"
                    style={{
                      background: '#10b98120',
                      border: '1px solid #10b98180',
                      color: '#10b981',
                      padding: '3px 10px',
                      borderRadius: 4,
                      fontSize: 12,
                      cursor: 'pointer',
                      transition: 'all .15s ease',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.background = '#10b98135'
                      e.currentTarget.style.borderColor = '#10b981'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = '#10b98120'
                      e.currentTarget.style.borderColor = '#10b98180'
                    }}
                  >
                    财务趋势
                  </button>
                  <button className="stock-info-refresh" onClick={handleRefreshProfile} title="强制刷新">
                    刷新
                  </button>
                </div>
              </div>
            </div>

            {/* 导入/导出操作区 */}
            <div className="actions-sub" style={{ marginBottom: 10, justifyContent: 'center', gap: 12 }}>
              <button className="btn-text" onClick={handleImport} disabled={loading}>
                {loading ? '处理中...' : '导入本地csv/excel财报'}
              </button>
              <button className="btn-text" onClick={handleExportCurrentData} disabled={!selectedStock || dataHistory.length === 0} title={dataHistory.length === 0 ? '请先下载或导入财报数据' : '导出当前财务数据到本地'}>
                导出本地财报
              </button>
            </div>

            {/* 主操作按钮 */}
            <div className="actions">
              <button className="btn primary" onClick={handleDownload} disabled={downloading || loading}>
                下载财报
              </button>
              <button className="btn primary" onClick={handleAnalyze} disabled={analyzing || downloading || loading || dataHistory.length === 0} title={dataHistory.length === 0 ? '请先下载或导入财报数据' : ''}>
                财报分析
              </button>
            </div>

            {/* 状态显示 - 按钮下方一行 */}
            <div className="action-status-line">
              {downloading && <span className="status-downloading">正在下载...</span>}
              {downloadStatus.type === 'success' && !downloading && (
                <span className="status-success" title={downloadStatus.message}>{downloadStatus.message}</span>
              )}
              {downloadStatus.type === 'error' && !downloading && (
                <span className="status-error" title={downloadStatus.message}>{downloadStatus.message.length > 30 ? downloadStatus.message.slice(0, 30) + '...' : downloadStatus.message}</span>
              )}
              {analyzing && (
                <span className="status-analyzing">
                  分析中 {analyzeProgress}%{analyzeProgress >= 90 ? '(获取行情/舆情/ML...)' : ''}
                </span>
              )}
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

            {currentSnapshot && (
              <Collapsible title="💡 亮点与风险">
                <div className="highlights-risks" style={{ marginTop: 0, marginBottom: 0 }}>
                  {highlights.length > 0 && (
                    <div className="hr-section">
                      {highlights.map((h, idx) => (
                        <div key={`h-${idx}`} className="highlight-item">
                          {h}
                        </div>
                      ))}
                    </div>
                  )}
                  {risks.length > 0 && (
                    <div className="hr-section">
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

            {selectedStock && (
              <Collapsible title="📊 行业对比雷达">
                <div className="risk-radar-collapsible-body" style={{ marginTop: 0, marginBottom: 0 }}>
                  {riskRadar && riskRadar.length > 0 ? (
                    <>
                      <table className="risk-radar-table">
                        <thead>
                          <tr>
                            <th style={{ width: 40, textAlign: 'center' }}>状态</th>
                            <th>指标</th>
                            <th style={{ textAlign: 'right' }}>当前值</th>
                            <th style={{ textAlign: 'right' }}>行业均值</th>
                          </tr>
                        </thead>
                        <tbody>
                          {riskRadar.map((item, idx) => (
                            <tr key={idx} className={`risk-radar-tr risk-radar-${item.level}`} title={item.desc}>
                              <td style={{ textAlign: 'center' }}>{item.icon}</td>
                              <td>{item.name}</td>
                              <td style={{ textAlign: 'right', fontWeight: 500 }}>{item.value}</td>
                              <td style={{ textAlign: 'right', color: '#94a3b8' }}>{item.industry || '-'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                      <div className="risk-radar-hint">基于本地数据计算 · 设置中可更新</div>
                    </>
                  ) : (
                    <div className="risk-radar-empty">暂无对比数据（请先执行财报分析）</div>
                  )}
                </div>
              </Collapsible>
            )}

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
                    {compDownloading ? '下载中...' : '下载财报'}
                  </button>
                  {(() => {
                    const compChanged = JSON.stringify([...appliedComparables].sort()) !== JSON.stringify([...comparables].sort())
                    const canUpdate = compReportsDownloaded && comparables.length > 0
                    return (
                      <button
                        className={`btn-icon cp-merge${compChanged ? ' changed' : ''}`}
                        title={canUpdate ? '更新模块4（行业横向对比分析）到报告' : '请先下载可比公司财报'}
                        onClick={handleAnalyzeWithComparables}
                        disabled={analyzing || !canUpdate}
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
                {compDownloadStatus.type && !compDownloading && (
                  <div className="cp-status-line">
                    {compDownloadStatus.type === 'success' ? (
                      <span className="status-success" title={compDownloadStatus.message}>{compDownloadStatus.message}</span>
                    ) : (
                      <span className="status-error" title={compDownloadStatus.message}>{compDownloadStatus.message.length > 40 ? compDownloadStatus.message.slice(0, 40) + '...' : compDownloadStatus.message}</span>
                    )}
                  </div>
                )}

              </div>
            </Collapsible>

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
            {selectedStock && (
              <span className="report-timestamp">
                {lastAnalysisAt
                  ? `上次分析: ${lastAnalysisAt}`
                  : '请先执行财报分析'}
              </span>
            )}
            {displayContent && (
              <select
                ref={tocSelectRef}
                className="toc-select"
                value=""
                onChange={(e) => {
                  const id = e.target.value
                  if (id) {
                    handleTocJump(id)
                    const select = e.target
                    const label = tocSections.find((s) => s.id === id)?.label || '📑 跳转章节'
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
            <div className="download-dropdown" ref={downloadMenuRef}>
              <button
                ref={downloadMenuBtnRef}
                className="btn-download"
                onClick={() => setDownloadMenuOpen(!downloadMenuOpen)}
                disabled={!displayContent}
                title={!displayContent ? '请先执行分析' : '下载当前显示的报告'}
              >
                下载报告 ▼
              </button>
              {downloadMenuOpen && (
                <div className="download-dropdown-menu">
                  <div
                    className="download-dropdown-item"
                    onClick={() => {
                      setDownloadMenuOpen(false)
                      handleReportDownload()
                    }}
                  >
                    <span>📝</span> Markdown 格式
                  </div>
                  <div
                    className="download-dropdown-item"
                    onClick={() => {
                      setDownloadMenuOpen(false)
                      handleExportPDF()
                    }}
                  >
                    <span>📄</span> PDF 格式
                  </div>
                  <div
                    className="download-dropdown-item"
                    onClick={() => {
                      setDownloadMenuOpen(false)
                      handleDownloadImage()
                    }}
                  >
                    <span>🖼️</span> 长图片
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
        <div className="report-content" ref={reportContentRef}>
          {displayContent ? (
            <div className="markdown-body" onClick={(e) => {
              const target = e.target as HTMLElement
              if (target.closest('.rim-adjust-btn')) {
                e.preventDefault()
                openRIMModal()
                return
              }
              if (target.closest('.fetch-activity-trigger')) {
                e.preventDefault()
                handleFetchMissingActivity()
              }
            }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSlug, rehypeRaw]} components={markdownComponents}>
                {displayContent}
              </ReactMarkdown>
            </div>
          ) : selectedStock ? (
            <div className="placeholder">
              <p>【Markdown 报告展示区】</p>
              <p>选择股票后点击"财报分析"，报告将在此渲染</p>
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
              <div className="rim-hint" style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 8 }}>
                💡 默认参数基准：2025年4月市场数据，建议根据当前市场环境调整
              </div>
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

      {/* 财务指标趋势图弹窗 */}
      {trendDrawerCode && (
        <FinancialTrendDrawer
          code={trendDrawerCode}
          name={selectedStock?.name}
          onClose={() => setTrendDrawerCode(null)}
        />
      )}
    </div>
  )
}

export default App
