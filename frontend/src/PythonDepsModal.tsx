import { useState, useEffect, useRef, useCallback } from 'react'
import './App.css'
import {
  CheckPythonDependencies,
  InstallPythonDependencies,
  MarkPythonDepsChecked,
} from '../wailsjs/go/main/App'
import type { main } from '../wailsjs/go/models'
import { EventsOn, EventsOff } from '../wailsjs/runtime'

type PythonEnvResult = main.PythonEnvResult

interface PythonDepsModalProps {
  isOpen: boolean
  onClose: () => void
}

export function PythonDepsModal({ isOpen, onClose }: PythonDepsModalProps) {
  const [result, setResult] = useState<PythonEnvResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [installing, setInstalling] = useState(false)
  const [logs, setLogs] = useState<string[]>([])
  const [installError, setInstallError] = useState('')
  const logEndRef = useRef<HTMLDivElement>(null)

  const scrollToBottom = () => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => {
    scrollToBottom()
  }, [logs])

  const check = useCallback(async () => {
    setLoading(true)
    setInstallError('')
    try {
      const r = await CheckPythonDependencies()
      setResult(r)
    } catch (e: any) {
      setInstallError('检测失败: ' + (e?.message || String(e)))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (isOpen) {
      check()
    }
  }, [isOpen, check])

  useEffect(() => {
    if (!installing) return
    const handler = (data: string) => {
      setLogs(prev => [...prev, data])
    }
    EventsOn('python:install:progress', handler)
    return () => {
      EventsOff('python:install:progress')
    }
  }, [installing])

  const handleInstall = async () => {
    if (!result || result.missing.length === 0) return
    setInstalling(true)
    setLogs([])
    setInstallError('')
    try {
      await InstallPythonDependencies(result.missing)
      // 安装完成后重新检测
      await check()
    } catch (e: any) {
      setInstallError('安装失败: ' + (e?.message || String(e)))
    } finally {
      setInstalling(false)
    }
  }

  const handleSkip = () => {
    MarkPythonDepsChecked()
    onClose()
  }

  if (!isOpen) return null

  const missingRequired = result?.packages?.filter(
    p => p.required && !p.installed
  ) || []
  const missingOptional = result?.packages?.filter(
    p => !p.required && !p.installed
  ) || []

  return (
    <div className="modal-overlay" onClick={e => {
      if (e.target === e.currentTarget && !installing) onClose()
    }}>
      <div className="modal-content" style={{ maxWidth: 480, width: '90%' }}>
        <h4 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span>🐍</span>
          Python 环境检测
        </h4>

        {loading && (
          <p>正在检测 Python 环境及依赖包...</p>
        )}

        {!loading && !result?.pythonFound && (
          <div>
            <p style={{ color: '#f87171' }}>
              ⚠️ 未检测到 Python 环境。本应用的部分功能（ML 预测、数据更新）需要 Python 3.10+ 支持。
            </p>
            <div style={{
              background: '#0f1720',
              borderRadius: 8,
              padding: 12,
              marginBottom: 16,
              fontSize: 13,
              lineHeight: 1.7,
            }}>
              <strong>Windows 安装步骤：</strong><br/>
              1. 访问 <a href="https://www.python.org/downloads/" target="_blank" rel="noopener noreferrer" style={{ color: '#60a5fa' }}>python.org</a> 下载 Python 3.10+<br/>
              2. 安装时勾选 "Add Python to PATH"<br/>
              3. 重启本应用后再次检测<br/><br/>
              <strong>macOS 安装步骤：</strong><br/>
              1. 运行 <code style={{ background: '#1e293b', padding: '2px 6px', borderRadius: 4 }}>brew install python@3.12</code><br/>
              2. 或从 python.org 下载安装包
            </div>
          </div>
        )}

        {!loading && result?.pythonFound && (
          <div>
            <p style={{ marginBottom: 8 }}>
              <span style={{ color: '#4ade80' }}>✓</span> Python 已找到: <code>{result.pythonPath}</code><br/>
              <span style={{ color: '#94a3b8', fontSize: 12 }}>{result.version}</span>
            </p>

            {result.packages && result.packages.length > 0 && (
              <div style={{
                background: '#0f1720',
                borderRadius: 8,
                padding: 12,
                marginBottom: 16,
                fontSize: 13,
              }}>
                {result.packages.map(pkg => (
                  <div key={pkg.name} style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '6px 0',
                    borderBottom: '1px solid #1f2933',
                  }}>
                    <span>
                      {pkg.installed ? (
                        <span style={{ color: '#4ade80' }}>✓</span>
                      ) : (
                        <span style={{ color: '#f87171' }}>✗</span>
                      )}
                      {' '}{pkg.display}
                      {pkg.required && (
                        <span style={{ color: '#fbbf24', fontSize: 11, marginLeft: 4 }}>必需</span>
                      )}
                    </span>
                    <span style={{ color: pkg.installed ? '#4ade80' : '#f87171', fontSize: 12 }}>
                      {pkg.installed ? (pkg.version || '已安装') : '未安装'}
                    </span>
                  </div>
                ))}
              </div>
            )}

            {result.allReady && (
              <p style={{ color: '#4ade80' }}>
                ✅ 所有核心依赖包已就绪，可以正常使用全部功能。
              </p>
            )}

            {!result.allReady && missingRequired.length > 0 && (
              <p style={{ color: '#fbbf24' }}>
                ⚠️ 缺少 {missingRequired.length} 个必需包，ML 预测和部分数据功能可能无法使用。
              </p>
            )}

            {!result.allReady && missingRequired.length === 0 && missingOptional.length > 0 && (
              <p style={{ color: '#fbbf24' }}>
                ℹ️ 缺少 {missingOptional.length} 个可选包，核心功能不受影响。
              </p>
            )}
          </div>
        )}

        {installing && (
          <div style={{
            background: '#0f1720',
            borderRadius: 8,
            padding: 12,
            marginBottom: 16,
            maxHeight: 200,
            overflowY: 'auto',
            fontSize: 12,
            fontFamily: 'monospace',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            color: '#94a3b8',
          }}>
            {logs.map((line, i) => (
              <div key={i}>{line}</div>
            ))}
            <div ref={logEndRef} />
          </div>
        )}

        {installError && (
          <p style={{ color: '#f87171', fontSize: 13, marginBottom: 12 }}>
            {installError}
          </p>
        )}

        <div className="modal-actions">
          {!loading && result && !result.pythonFound && (
            <button
              className="btn btn-secondary"
              onClick={handleSkip}
            >
              跳过（下次启动再提醒）
            </button>
          )}

          {!loading && result?.pythonFound && !result.allReady && (
            <>
              <button
                className="btn btn-secondary"
                onClick={handleSkip}
                disabled={installing}
              >
                跳过
              </button>
              <button
                className="btn btn-primary"
                onClick={handleInstall}
                disabled={installing || result.missing.length === 0}
              >
                {installing ? '安装中...' : '一键安装缺失包'}
              </button>
            </>
          )}

          {!loading && result?.allReady && (
            <button
              className="btn btn-primary"
              onClick={() => { MarkPythonDepsChecked(); onClose() }}
            >
              开始使用
            </button>
          )}

          <button
            className="btn btn-secondary"
            onClick={check}
            disabled={loading || installing}
          >
            重新检测
          </button>
        </div>
      </div>
    </div>
  )
}
