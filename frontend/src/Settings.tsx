import { useState, useEffect, useRef, useCallback } from 'react'
import './Settings.css'
import { GetTushareConfig, SaveTushareConfig, VerifyTushareToken } from '../wailsjs/go/main/App'
import type { main } from '../wailsjs/go/models'

export interface AppSettings {
  theme: 'dark' | 'light' | 'system'
  klineDefaultRange: '1m' | '3m' | '6m' | '1y' | 'all'
  showMA5: boolean
  showMA30: boolean
  showMA180: boolean
  showMA250: boolean
  reportYears: number
  autoUpdateIndustryDB: boolean
  analysisNotification: boolean
  riskSensitivity: 'strict' | 'standard' | 'loose'
}

const DEFAULT_SETTINGS: AppSettings = {
  theme: 'dark',
  klineDefaultRange: '6m',
  showMA5: true,
  showMA30: true,
  showMA180: true,
  showMA250: true,
  reportYears: 5,
  autoUpdateIndustryDB: true,
  analysisNotification: true,
  riskSensitivity: 'standard',
}

const SETTINGS_KEY = 'stockfinlens-settings-v1'

export function loadSettings(): AppSettings {
  try {
    const saved = localStorage.getItem(SETTINGS_KEY)
    if (saved) {
      return { ...DEFAULT_SETTINGS, ...JSON.parse(saved) }
    }
  } catch {
    // ignore
  }
  return DEFAULT_SETTINGS
}

export function saveSettings(settings: AppSettings) {
  try {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
  } catch {
    // ignore
  }
}

// 数据管理相关类型
interface PolicyLibMeta {
  version: string
  updatedAt: string
}

interface IndustryDBMeta {
  version: string
  updatedAt: string
  count: number
}

interface SettingsProps {
  settings: AppSettings
  onSettingsChange: (settings: AppSettings) => void
  // 数据管理相关
  policyLibMeta?: PolicyLibMeta | null
  industryDBMeta?: IndustryDBMeta | null
  policyUpdating?: boolean
  industryUpdating?: boolean
  onUpdatePolicyLibrary?: () => void
  onUpdateIndustryDB?: () => void
  policyActionStatus?: { type: 'success' | 'error' | null; message: string }
  industryActionStatus?: { type: 'success' | 'error' | null; message: string }
  industryTask?: any
  // Python 依赖检测
  onCheckPythonDeps?: () => void
}

export function Settings({ 
  settings, 
  onSettingsChange,
  policyLibMeta,
  industryDBMeta,
  policyUpdating = false,
  industryUpdating = false,
  onUpdatePolicyLibrary,
  onUpdateIndustryDB,
  policyActionStatus,
  industryActionStatus,
  industryTask,
  onCheckPythonDeps,
}: SettingsProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'appearance' | 'data' | 'about'>('appearance')
  const dropdownRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  // StockFinLens 数据源配置状态
  const [tushareCfg, setTushareCfg] = useState<main.TushareConfig | null>(null)
  const [tushareLoading, setTushareLoading] = useState(false)
  const [tushareVerifyStatus, setTushareVerifyStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [tushareSaving, setTushareSaving] = useState(false)

  // 加载数据源配置
  useEffect(() => {
    GetTushareConfig().then((cfg) => {
      setTushareCfg(cfg)
    }).catch(() => {
      setTushareCfg({
        enabled: false, token: '', verified: false, verified_at: '',
        use_for_financial: true, use_for_kline: true, use_for_quote: true, use_for_moneyflow: true
      } as main.TushareConfig)
    })
  }, [isOpen])

  const handleVerifyTushare = useCallback(async () => {
    if (!tushareCfg?.token) return
    setTushareVerifyStatus({type: null, message: ''})
    setTushareLoading(true)
    try {
      const result = await VerifyTushareToken(tushareCfg.token)
      setTushareVerifyStatus({type: result.success ? 'success' : 'error', message: result.message || '验证失败'})
      if (result.success) {
        setTushareCfg(prev => prev ? {...prev, verified: true, verified_at: new Date().toISOString()} : prev)
      }
    } catch (err: any) {
      setTushareVerifyStatus({type: 'error', message: err?.message || '验证失败'})
    } finally {
      setTushareLoading(false)
    }
  }, [tushareCfg?.token])

  const handleSaveTushare = useCallback(async () => {
    if (!tushareCfg) return
    setTushareSaving(true)
    try {
      await SaveTushareConfig(tushareCfg)
      setTushareVerifyStatus({type: 'success', message: '配置已保存'})
    } catch (err: any) {
      setTushareVerifyStatus({type: 'error', message: err?.message || '保存失败'})
    } finally {
      setTushareSaving(false)
    }
  }, [tushareCfg])

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const updateSetting = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    const newSettings = { ...settings, [key]: value }
    onSettingsChange(newSettings)
    saveSettings(newSettings)
  }

  const version = '1.3.29'

  return (
    <>
      <button
        ref={buttonRef}
        className="settings-toggle"
        title="设置"
        onClick={() => setIsOpen(!isOpen)}
      >
        ⚙️
      </button>

      {isOpen && (
        <div ref={dropdownRef} className="settings-dropdown">
          <div className="settings-tabs">
            <button className={activeTab === 'appearance' ? 'active' : ''} onClick={() => setActiveTab('appearance')}>外观</button>
            <button className={activeTab === 'data' ? 'active' : ''} onClick={() => setActiveTab('data')}>数据</button>
            <button className={activeTab === 'about' ? 'active' : ''} onClick={() => setActiveTab('about')}>关于</button>
          </div>

          {activeTab === 'appearance' && (
            <div className="settings-section">
              <div className="settings-item">
                <label>主题</label>
                <div className="settings-options">
                  <label className="settings-radio"><input type="radio" name="theme" checked={settings.theme === 'dark'} onChange={() => updateSetting('theme', 'dark')} /><span>深色</span></label>
                  <label className="settings-radio"><input type="radio" name="theme" checked={settings.theme === 'light'} onChange={() => updateSetting('theme', 'light')} /><span>浅色</span></label>
                  <label className="settings-radio"><input type="radio" name="theme" checked={settings.theme === 'system'} onChange={() => updateSetting('theme', 'system')} /><span>跟随系统</span></label>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'data' && (
            <div className="settings-section">
              {/* 基础数据设置 */}
              <div className="settings-item settings-item-inline">
                <label>财报下载年限</label>
                <div className="settings-input-group">
                  <input type="number" min={3} max={10} value={settings.reportYears} onChange={(e) => updateSetting('reportYears', parseInt(e.target.value) || 5)} />
                  <span>年</span>
                </div>
              </div>
              <div className="settings-item settings-item-inline">
                <label>自动更新行业库</label>
                <div className="settings-toggle-switch">
                  <label className="switch"><input type="checkbox" checked={settings.autoUpdateIndustryDB} onChange={(e) => updateSetting('autoUpdateIndustryDB', e.target.checked)} /><span className="slider"></span></label>
                </div>
              </div>
              <div className="settings-item settings-item-inline">
                <label>分析完成提示</label>
                <div className="settings-toggle-switch">
                  <label className="switch"><input type="checkbox" checked={settings.analysisNotification} onChange={(e) => updateSetting('analysisNotification', e.target.checked)} /><span className="slider"></span></label>
                </div>
              </div>

              {/* 风险警示敏感度 */}
              <div className="settings-item settings-item-inline">
                <label>风险警示敏感度</label>
                <div className="settings-input-group">
                  <select
                    value={settings.riskSensitivity}
                    onChange={(e) => updateSetting('riskSensitivity', e.target.value as 'strict' | 'standard' | 'loose')}
                    style={{ padding: '4px 8px', borderRadius: 4, border: '1px solid rgba(148,163,184,0.3)', background: 'rgba(15,23,42,0.6)', color: '#e2e8f0', fontSize: 13 }}
                  >
                    <option value="strict">严格</option>
                    <option value="standard">标准（默认）</option>
                    <option value="loose">宽松</option>
                  </select>
                </div>
              </div>

              {/* 数据管理分割线 */}
              <div className="settings-divider" />

              {/* 产业政策库 */}
              <div className="settings-data-section">
                <div className="settings-data-title">📚 产业政策库</div>
                <div className="settings-data-info">
                  <div>版本: <span>{policyLibMeta?.version || 'builtin'}</span></div>
                  <div>更新于: <span>{policyLibMeta?.updatedAt || '内置默认'}</span></div>
                </div>
                <div className="settings-data-desc">
                  为报告模块5（政策匹配度评估）提供政策关键词数据
                </div>
                {onUpdatePolicyLibrary && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onUpdatePolicyLibrary}
                    disabled={policyUpdating}
                  >
                    {policyUpdating ? '更新中...' : '🔄 更新政策库'}
                  </button>
                )}
                {policyActionStatus?.type && !policyUpdating && (
                  <div className="settings-action-status">
                    {policyActionStatus.type === 'success' ? (
                      <span className="status-success">{policyActionStatus.message}</span>
                    ) : (
                      <span className="status-error">{policyActionStatus.message.length > 40 ? policyActionStatus.message.slice(0, 40) + '...' : policyActionStatus.message}</span>
                    )}
                  </div>
                )}
              </div>

              {/* 行业均值数据库 */}
              <div className="settings-data-section">
                <div className="settings-data-title">🏭 行业均值数据库</div>
                <div className="settings-data-info">
                  <div>行业数: <span>{industryDBMeta?.count || 0}</span></div>
                  <div>更新于: <span>{industryDBMeta?.updatedAt || '未更新'}</span></div>
                </div>
                <div className="settings-data-desc">
                  为报告模块4（行业横向对比）提供行业基准数据
                </div>
                {onUpdateIndustryDB && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onUpdateIndustryDB}
                    disabled={industryUpdating}
                  >
                    {industryUpdating
                      ? (industryTask?.status === 'running' && industryTask?.total
                          ? `后台采集中 ${Math.round((industryTask.progress || 0) / industryTask.total * 100)}%...`
                          : '后台采集中...')
                      : '🔄 更新行业数据库'}
                  </button>
                )}
                {industryTask?.status === 'running' && (
                  <div className="settings-action-status">
                    <span style={{ color: '#94a3b8' }}>{industryTask.message || '正在采集全市场数据...'}</span>
                  </div>
                )}
                {industryTask?.status === 'completed' && !industryUpdating && (
                  <div className="settings-action-status">
                    <span className="status-success">{industryTask.message || '后台采集完成'}</span>
                  </div>
                )}
                {industryTask?.status === 'error' && !industryUpdating && (
                  <div className="settings-action-status">
                    <span className="status-error">{industryTask.message || '后台采集失败'}</span>
                  </div>
                )}
                {industryActionStatus?.type && !industryUpdating && industryTask?.status !== 'running' && (
                  <div className="settings-action-status">
                    {industryActionStatus.type === 'success' ? (
                      <span className="status-success">{industryActionStatus.message}</span>
                    ) : (
                      <span className="status-error">{industryActionStatus.message.length > 40 ? industryActionStatus.message.slice(0, 40) + '...' : industryActionStatus.message}</span>
                    )}
                  </div>
                )}
              </div>

              {/* StockFinLens 数据源 */}
              <div className="settings-data-section">
                <div className="settings-data-title">📊 StockFinLens 数据源</div>
                <div className="settings-data-desc">
                  启用 StockFinLens 数据源，提升数据稳定性
                </div>

                {tushareCfg && (
                  <>
                    <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                      <label>启用 StockFinLens</label>
                      <div className="settings-toggle-switch">
                        <label className="switch">
                          <input
                            type="checkbox"
                            checked={tushareCfg.enabled}
                            onChange={(e) => setTushareCfg({ ...tushareCfg, enabled: e.target.checked })}
                          />
                          <span className="slider"></span>
                        </label>
                      </div>
                    </div>

                    {tushareCfg.enabled && (
                      <>
                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>授权码</label>
                          <input
                            type="password"
                            value={tushareCfg.token}
                            onChange={(e) => setTushareCfg({ ...tushareCfg, token: e.target.value, verified: false })}
                            placeholder="请输入授权码"
                            style={{ width: '100%', marginTop: 4 }}
                          />
                          {tushareCfg.verified && (
                            <div style={{ fontSize: 11, color: '#22c55e', marginTop: 2 }}>
                              ✅ 已验证{tushareCfg.verified_at ? ` · ${tushareCfg.verified_at.slice(0, 10)}` : ''}
                            </div>
                          )}
                        </div>

                        <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                          <button
                            className="settings-data-btn"
                            onClick={handleVerifyTushare}
                            disabled={tushareLoading || !tushareCfg.token}
                          >
                            {tushareLoading ? '验证中...' : '🔍 验证连通性'}
                          </button>
                          <button
                            className="settings-data-btn"
                            onClick={handleSaveTushare}
                            disabled={tushareSaving}
                          >
                            {tushareSaving ? '保存中...' : '💾 保存配置'}
                          </button>
                        </div>

                        {tushareVerifyStatus.type && (
                          <div className="settings-action-status">
                            {tushareVerifyStatus.type === 'success' ? (
                              <span className="status-success">{tushareVerifyStatus.message}</span>
                            ) : (
                              <span className="status-error">{tushareVerifyStatus.message}</span>
                            )}
                          </div>
                        )}

                        <div className="settings-item" style={{ marginTop: 8, fontSize: 12 }}>
                          <label style={{ marginBottom: 4 }}>启用范围</label>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            <label><input type="checkbox" checked={tushareCfg.use_for_financial} onChange={(e) => setTushareCfg({ ...tushareCfg, use_for_financial: e.target.checked })} /> 财报数据</label>
                            <label><input type="checkbox" checked={tushareCfg.use_for_kline} onChange={(e) => setTushareCfg({ ...tushareCfg, use_for_kline: e.target.checked })} /> 历史K线</label>
                            <label><input type="checkbox" checked={tushareCfg.use_for_quote} onChange={(e) => setTushareCfg({ ...tushareCfg, use_for_quote: e.target.checked })} /> 每日指标（PE/PB/市值）</label>
                            <label><input type="checkbox" checked={tushareCfg.use_for_moneyflow} onChange={(e) => setTushareCfg({ ...tushareCfg, use_for_moneyflow: e.target.checked })} /> 个股资金流向</label>
                          </div>
                        </div>
                      </>
                    )}
                  </>
                )}
              </div>

            </div>
          )}

          {activeTab === 'about' && (
            <div className="settings-section about-section">
              <img src="/logo.png" className="about-logo" alt="StockFinLens Logo" />
              <div className="about-title">股票财报透镜</div>
              <div className="about-version">版本 {version}</div>
              <div className="about-desc">穿透财报看真相，自动扫描财务风险，重要指标可溯源。</div>
              <a href="https://github.com/liusaipu/stockfinlens/releases" target="_blank" rel="noopener noreferrer" className="about-link">检查更新</a>

              {/* 运行环境 */}
              <div className="settings-divider" style={{ margin: '16px 0' }} />
              <div className="settings-data-section" style={{ textAlign: 'left' }}>
                <div className="settings-data-title">🖥️ 运行环境</div>
                <div className="settings-data-desc">
                  检测 ML 推理和数据更新所需的 Python 依赖包
                </div>
                {onCheckPythonDeps && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onCheckPythonDeps}
                  >
                    🔍 检测运行环境
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </>
  )
}
