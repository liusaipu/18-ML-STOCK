import { useState, useEffect, useRef } from 'react'
import './Settings.css'

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
}

const SETTINGS_KEY = 'stock-analyzer-settings-v1'

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
  industryTask
}: SettingsProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'appearance' | 'data' | 'about'>('appearance')
  const dropdownRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

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

  const version = '1.3.25'

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
              <div className="settings-item">
                <label>财报下载年限</label>
                <div className="settings-input-group">
                  <input type="number" min={3} max={10} value={settings.reportYears} onChange={(e) => updateSetting('reportYears', parseInt(e.target.value) || 5)} />
                  <span>�?/span>
                </div>
              </div>
              <div className="settings-item">
                <label>自动更新行业�?/label>
                <div className="settings-toggle-switch">
                  <label className="switch"><input type="checkbox" checked={settings.autoUpdateIndustryDB} onChange={(e) => updateSetting('autoUpdateIndustryDB', e.target.checked)} /><span className="slider"></span></label>
                </div>
              </div>
              <div className="settings-item">
                <label>分析完成提示</label>
                <div className="settings-toggle-switch">
                  <label className="switch"><input type="checkbox" checked={settings.analysisNotification} onChange={(e) => updateSetting('analysisNotification', e.target.checked)} /><span className="slider"></span></label>
                </div>
              </div>

              {/* 数据管理分割�?*/}
              <div className="settings-divider" />

              {/* 产业政策�?*/}
              <div className="settings-data-section">
                <div className="settings-data-title">📚 产业政策�?/div>
                <div className="settings-data-info">
                  <div>版本: <span>{policyLibMeta?.version || 'builtin'}</span></div>
                  <div>更新�? <span>{policyLibMeta?.updatedAt || '内置默认'}</span></div>
                </div>
                <div className="settings-data-desc">
                  为报告模�?（政策匹配度评估）提供政策关键词数据
                </div>
                {onUpdatePolicyLibrary && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onUpdatePolicyLibrary}
                    disabled={policyUpdating}
                  >
                    {policyUpdating ? '更新�?..' : '🔄 更新政策�?}
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
                  <div>行业�? <span>{industryDBMeta?.count || 0}</span></div>
                  <div>更新�? <span>{industryDBMeta?.updatedAt || '未更�?}</span></div>
                </div>
                <div className="settings-data-desc">
                  为报告模�?（行业横向对比）提供行业基准数据
                </div>
                {onUpdateIndustryDB && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onUpdateIndustryDB}
                    disabled={industryUpdating}
                  >
                    {industryUpdating
                      ? (industryTask?.status === 'running' && industryTask?.total
                          ? `后台采集�?${Math.round((industryTask.progress || 0) / industryTask.total * 100)}%...`
                          : '后台采集�?..')
                      : '🔄 更新行业数据�?}
                  </button>
                )}
                {industryTask?.status === 'running' && (
                  <div className="settings-action-status">
                    <span style={{ color: '#94a3b8' }}>{industryTask.message || '正在采集全市场数�?..'}</span>
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
            </div>
          )}

          {activeTab === 'about' && (
            <div className="settings-section about-section">
              <img src="/logo.png" className="about-logo" alt="StockFinLens Logo" />
              <div className="about-title">股票财报透镜</div>
              <div className="about-version">版本 {version}</div>
              <div className="about-desc">穿透财报看真相，自动扫描财务风险，重要指标可溯源�?/div>
              <a href="https://github.com/liusaipu/stockfinlens/releases" target="_blank" rel="noopener noreferrer" className="about-link">检查更�?/a>
            </div>
          )}
        </div>
      )}
    </>
  )
}
