import { useState, useEffect, useRef, useCallback } from 'react'
import './Settings.css'
import { GetSFLConfig, SaveSFLConfig, VerifySFLToken, CheckForUpdate, SetAutoCheckUpdate } from '../wailsjs/go/main/App'
import type { main } from '../wailsjs/go/models'
import { UpdateModal } from './UpdateModal'

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
  autoCheckUpdate: boolean
}

export const DEFAULT_SETTINGS: AppSettings = {
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
  autoCheckUpdate: true,
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
  const [sflCfg, setSflCfg] = useState<main.SFLConfig | null>(null)
  const [sflLoading, setSflLoading] = useState(false)
  const [sflVerifyStatus, setSflVerifyStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [sflSaving, setSflSaving] = useState(false)

  // 检查更新状态
  const [updateChecking, setUpdateChecking] = useState(false)
  const [updateCheckResult, setUpdateCheckResult] = useState<{type: 'success' | 'info' | 'error' | null, message: string}>({type: null, message: ''})
  const [showUpdateModal, setShowUpdateModal] = useState(false)
  const [foundUpdateInfo, setFoundUpdateInfo] = useState<any>(null)

  // 加载数据源配置
  useEffect(() => {
    GetSFLConfig().then((cfg) => {
      setSflCfg(cfg)
    }).catch(() => {
      setSflCfg({
        enabled: false, token: '', verified: false, verified_at: '',
        use_for_financial: true, use_for_kline: true, use_for_quote: true, use_for_moneyflow: true,
        moneyflow_days: 3
      } as main.SFLConfig)
    })
  }, [isOpen])

  const handleVerifySFL = useCallback(async () => {
    if (!sflCfg?.token) return
    setSflVerifyStatus({type: null, message: ''})
    setSflLoading(true)
    try {
      const result = await VerifySFLToken(sflCfg.token)
      setSflVerifyStatus({type: result.success ? 'success' : 'error', message: result.message || '验证失败'})
      if (result.success) {
        setSflCfg(prev => prev ? {...prev, verified: true, verified_at: new Date().toISOString()} : prev)
      }
    } catch (err: any) {
      setSflVerifyStatus({type: 'error', message: err?.message || '验证失败'})
    } finally {
      setSflLoading(false)
    }
  }, [sflCfg?.token])

  const handleSaveSFL = useCallback(async () => {
    if (!sflCfg) return
    setSflSaving(true)
    try {
      await SaveSFLConfig(sflCfg)
      setSflVerifyStatus({type: 'success', message: '配置已保存'})
    } catch (err: any) {
      setSflVerifyStatus({type: 'error', message: err?.message || '保存失败'})
    } finally {
      setSflSaving(false)
    }
  }, [sflCfg])

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

  const version = '1.3.38'

  const handleCheckUpdate = useCallback(async () => {
    setUpdateChecking(true)
    setUpdateCheckResult({type: null, message: ''})
    try {
      const info = await CheckForUpdate()
      if (info.hasUpdate) {
        setUpdateCheckResult({type: 'info', message: `发现新版本 ${info.latestVer}`})
        setFoundUpdateInfo(info)
        setShowUpdateModal(true)
      } else {
        setUpdateCheckResult({type: 'success', message: '当前已是最新版本'})
      }
    } catch (err: any) {
      setUpdateCheckResult({type: 'error', message: err?.message || '检查失败'})
    } finally {
      setUpdateChecking(false)
    }
  }, [])

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

      <UpdateModal
        isOpen={showUpdateModal}
        info={foundUpdateInfo}
        onClose={() => setShowUpdateModal(false)}
      />

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

                {sflCfg && (
                  <>
                    <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                      <label>启用 StockFinLens</label>
                      <div className="settings-toggle-switch">
                        <label className="switch">
                          <input
                            type="checkbox"
                            checked={sflCfg.enabled}
                            onChange={(e) => setSflCfg({ ...sflCfg, enabled: e.target.checked })}
                          />
                          <span className="slider"></span>
                        </label>
                      </div>
                    </div>

                    {sflCfg.enabled && (
                      <>
                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>授权码</label>
                          <input
                            type="password"
                            value={sflCfg.token}
                            onChange={(e) => setSflCfg({ ...sflCfg, token: e.target.value, verified: false })}
                            placeholder="请输入授权码"
                            style={{ width: '100%', marginTop: 4 }}
                          />
                          {sflCfg.verified && (
                            <div style={{ fontSize: 11, color: '#22c55e', marginTop: 2 }}>
                              ✅ 已验证{sflCfg.verified_at ? ` · ${sflCfg.verified_at.slice(0, 10)}` : ''}
                            </div>
                          )}
                        </div>

                        {sflVerifyStatus.type && (
                          <div className="settings-action-status" style={{ marginTop: 8 }}>
                            {sflVerifyStatus.type === 'success' ? (
                              <span className="status-success">{sflVerifyStatus.message}</span>
                            ) : (
                              <span className="status-error">{sflVerifyStatus.message}</span>
                            )}
                          </div>
                        )}

                        <div className="settings-item" style={{ marginTop: 8, fontSize: 12 }}>
                          <label style={{ marginBottom: 4 }}>启用范围</label>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            <label><input type="checkbox" checked={sflCfg.use_for_financial} onChange={(e) => setSflCfg({ ...sflCfg, use_for_financial: e.target.checked })} /> 财报数据</label>
                            <label><input type="checkbox" checked={sflCfg.use_for_kline} onChange={(e) => setSflCfg({ ...sflCfg, use_for_kline: e.target.checked })} /> 历史K线</label>
                            <label><input type="checkbox" checked={sflCfg.use_for_quote} onChange={(e) => setSflCfg({ ...sflCfg, use_for_quote: e.target.checked })} /> 每日指标（PE/PB/市值）</label>
                            <label style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                              <input type="checkbox" checked={sflCfg.use_for_moneyflow} onChange={(e) => setSflCfg({ ...sflCfg, use_for_moneyflow: e.target.checked })} />
                              <span>个股资金流向</span>
                              {sflCfg.enabled && sflCfg.use_for_moneyflow && (
                                <span style={{ display: 'flex', alignItems: 'center', gap: 4, marginLeft: 4 }}>
                                  <input
                                    type="number"
                                    min={3}
                                    max={10}
                                    value={sflCfg.moneyflow_days || 3}
                                    onChange={(e) => {
                                      const val = parseInt(e.target.value, 10)
                                      if (!isNaN(val) && val >= 3 && val <= 10) {
                                        setSflCfg({ ...sflCfg, moneyflow_days: val })
                                      }
                                    }}
                                    style={{
                                      width: 36,
                                      height: 20,
                                      fontSize: 11,
                                      textAlign: 'center',
                                      borderRadius: 3,
                                      border: '1px solid #cbd5e1',
                                      background: '#f8fafc',
                                      color: '#334155',
                                    }}
                                  />
                                  <span style={{ fontSize: 11, color: '#64748b' }}>个交易日</span>
                                </span>
                              )}
                            </label>
                          </div>
                        </div>

                        {/* 操作按钮放在启用范围下方 */}
                        <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
                          <button
                            className="settings-data-btn"
                            onClick={handleVerifySFL}
                            disabled={sflLoading || !sflCfg.token}
                            style={{ whiteSpace: 'nowrap' }}
                          >
                            {sflLoading ? '验证中...' : '🔍 验证连通性'}
                          </button>
                          <button
                            className="settings-data-btn"
                            onClick={handleSaveSFL}
                            disabled={sflSaving}
                            style={{ whiteSpace: 'nowrap' }}
                          >
                            {sflSaving ? '保存中...' : '💾 保存配置'}
                          </button>
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
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 16, marginTop: 8 }}>
                <button
                  className="about-link"
                  onClick={handleCheckUpdate}
                  disabled={updateChecking}
                  style={{
                    background: 'none',
                    border: 'none',
                    color: '#60a5fa',
                    cursor: updateChecking ? 'not-allowed' : 'pointer',
                    textDecoration: 'underline',
                    fontSize: 13,
                    padding: 0,
                  }}
                >
                  {updateChecking ? '检查中...' : '检查更新'}
                </button>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: '#94a3b8', cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={settings.autoCheckUpdate}
                    onChange={async (e) => {
                      const checked = e.target.checked
                      updateSetting('autoCheckUpdate', checked)
                      try { await SetAutoCheckUpdate(checked) } catch { /* ignore */ }
                    }}
                    style={{ cursor: 'pointer' }}
                  />
                  启动时自动检查
                </label>
              </div>
              {updateCheckResult.type && (
                <div style={{
                  fontSize: 12,
                  marginTop: 6,
                  color: updateCheckResult.type === 'success' ? '#4ade80' : updateCheckResult.type === 'error' ? '#f87171' : '#fbbf24',
                }}>
                  {updateCheckResult.type === 'success' ? '✓ ' : updateCheckResult.type === 'error' ? '✗ ' : 'ℹ️ '}
                  {updateCheckResult.message}
                </div>
              )}

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
