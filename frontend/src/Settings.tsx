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
  industryActionStatus
}: SettingsProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'appearance' | 'chart' | 'data' | 'about'>('appearance')
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

  const version = '1.3.18'

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
            <button className={activeTab === 'chart' ? 'active' : ''} onClick={() => setActiveTab('chart')}>图表</button>
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

          {activeTab === 'chart' && (
            <div className="settings-section">
              <div className="settings-item">
                <label>K线默认时间范围</label>
                <select value={settings.klineDefaultRange} onChange={(e) => updateSetting('klineDefaultRange', e.target.value as AppSettings['klineDefaultRange'])}>
                  <option value="1m">1个月</option>
                  <option value="3m">3个月</option>
                  <option value="6m">6个月</option>
                  <option value="1y">1年</option>
                  <option value="all">全部</option>
                </select>
              </div>
              <div className="settings-item">
                <label>均线显示</label>
                <div className="settings-checkboxes">
                  <label className="settings-checkbox"><input type="checkbox" checked={settings.showMA5} onChange={(e) => updateSetting('showMA5', e.target.checked)} /><span>MA5</span></label>
                  <label className="settings-checkbox"><input type="checkbox" checked={settings.showMA30} onChange={(e) => updateSetting('showMA30', e.target.checked)} /><span>MA30</span></label>
                  <label className="settings-checkbox"><input type="checkbox" checked={settings.showMA180} onChange={(e) => updateSetting('showMA180', e.target.checked)} /><span>MA180</span></label>
                  <label className="settings-checkbox"><input type="checkbox" checked={settings.showMA250} onChange={(e) => updateSetting('showMA250', e.target.checked)} /><span>MA250(年线)</span></label>
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
                  <span>年</span>
                </div>
              </div>
              <div className="settings-item">
                <label>自动更新行业库</label>
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
                    {industryUpdating ? '更新中(约2-3分钟)...' : '🔄 更新行业数据库'}
                  </button>
                )}
                {industryActionStatus?.type && !industryUpdating && (
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
              <div className="about-logo">📈</div>
              <div className="about-title">股票分析系统</div>
              <div className="about-version">版本 {version}</div>
              <div className="about-desc">基于18步财报分析框架的股票研究工具</div>
              <a href="https://github.com/liusaipu/18-ML-STOCK/releases" target="_blank" rel="noopener noreferrer" className="about-link">检查更新</a>
            </div>
          )}
        </div>
      )}
    </>
  )
}
