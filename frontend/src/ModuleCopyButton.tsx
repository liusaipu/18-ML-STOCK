import { useState, useRef, useEffect } from 'react'
import { toPng } from 'html-to-image'
import './ModuleCopyButton.css'

interface ModuleCopyButtonProps {
  moduleId: string
  moduleTitle: string
}

// 存储原始 Markdown 内容
let globalMarkdownContent: string = ''

export function setGlobalMarkdownContent(content: string) {
  globalMarkdownContent = content
}

export function ModuleCopyButton({ moduleId, moduleTitle }: ModuleCopyButtonProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [toast, setToast] = useState<string | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  // 点击外部关闭菜单
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        menuRef.current &&
        !menuRef.current.contains(e.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // 显示提示
  const showToast = (message: string) => {
    setToast(message)
    setTimeout(() => setToast(null), 2000)
  }

  // 获取当前模块的 DOM 元素（标题）
  const getModuleHeading = (): HTMLElement | null => {
    if (moduleId) {
      const el = document.getElementById(moduleId)
      if (el) return el
    }
    // 通过标题文本查找
    const headings = document.querySelectorAll('h1, h2')
    for (const h of headings) {
      if (h.textContent?.includes(moduleTitle.replace(/[\*\s]/g, ''))) {
        return h as HTMLElement
      }
    }
    return null
  }

  // 获取模块内容范围（从当前标题到下一个同级或更高级标题）
  const getModuleContentElement = (): HTMLElement | null => {
    const heading = getModuleHeading()
    if (!heading) return null

    // 创建临时容器
    const wrapper = document.createElement('div')
    wrapper.style.padding = '20px'
    wrapper.style.background = getComputedStyle(document.body).backgroundColor
    wrapper.style.color = getComputedStyle(document.body).color
    wrapper.style.fontFamily = getComputedStyle(document.body).fontFamily
    wrapper.style.fontSize = '14px'
    wrapper.style.lineHeight = '1.6'

    // 添加表格样式
    const styleEl = document.createElement('style')
    styleEl.textContent = `
      table {
        border-collapse: collapse;
        width: 100%;
        margin: 12px 0;
      }
      th, td {
        border: 1px solid #3a4a5a;
        padding: 8px 12px;
        text-align: left;
      }
      th {
        background: #1a2a3a;
        font-weight: 600;
      }
      tr:nth-child(even) {
        background: rgba(255,255,255,0.03);
      }
    `
    wrapper.appendChild(styleEl)

    // 克隆标题（移除复制按钮）
    const titleClone = heading.cloneNode(true) as HTMLElement
    const btn = titleClone.querySelector('.module-copy-wrapper')
    if (btn) btn.remove()
    // 移除 inline style
    titleClone.style.display = ''
    titleClone.style.alignItems = ''
    titleClone.style.flexWrap = ''
    wrapper.appendChild(titleClone)

    // 获取当前标题级别
    const currentLevel = heading.tagName // H1 or H2

    // 获取后续内容直到下一个同级或更高级标题
    let next = heading.nextElementSibling
    while (next) {
      // 如果遇到同级或更高级标题，停止
      if (next.tagName === 'H1' || (currentLevel === 'H2' && next.tagName === 'H2')) {
        break
      }
      // 克隆元素
      const clone = next.cloneNode(true) as HTMLElement
      // 移除其中的复制按钮
      const btns = clone.querySelectorAll('.module-copy-wrapper')
      btns.forEach(b => b.remove())
      wrapper.appendChild(clone)
      next = next.nextElementSibling
    }

    return wrapper
  }

  // 从全局 Markdown 内容中提取当前模块
  const getModuleMarkdown = (): string => {
    if (!globalMarkdownContent) return ''

    const lines = globalMarkdownContent.split('\n')
    let startIdx = -1
    let endIdx = lines.length

    // 找到模块标题行
    for (let i = 0; i < lines.length; i++) {
      const line = lines[i]
      // 匹配模块标题（# 模块X: 或 ## X.Y）
      if (line.match(/^#{1,2}\s+/) && line.includes(moduleTitle.replace(/\*/g, ''))) {
        startIdx = i
        // 找到下一个模块标题
        for (let j = i + 1; j < lines.length; j++) {
          const nextLine = lines[j]
          // 下一个 h1 或同级 h2
          if (nextLine.match(/^#{1,2}\s+/) && 
              (nextLine.match(/^#\s+/) || nextLine.match(/^##\s+\d+\./))) {
            endIdx = j
            break
          }
        }
        break
      }
    }

    if (startIdx === -1) return ''
    return lines.slice(startIdx, endIdx).join('\n')
  }

  // 复制 Markdown
  const copyMarkdown = async () => {
    const markdown = getModuleMarkdown()
    if (!markdown) {
      showToast('复制失败：未找到模块内容')
      return
    }

    try {
      await navigator.clipboard.writeText(markdown)
      showToast('已复制 Markdown 格式')
    } catch {
      showToast('复制失败')
    }
    setIsOpen(false)
  }

  // 复制纯文本
  const copyPlainText = async () => {
    const el = getModuleContentElement()
    if (!el) {
      showToast('复制失败：未找到模块内容')
      return
    }

    // 获取纯文本内容
    const plainText = el.innerText
      .replace(/\n{3,}/g, '\n\n') // 规范化空行
      .trim()

    try {
      await navigator.clipboard.writeText(plainText)
      showToast('已复制纯文本')
    } catch {
      showToast('复制失败')
    }
    setIsOpen(false)
  }

  // 复制为图片
  const copyAsImage = async () => {
    const el = getModuleContentElement()
    if (!el) {
      showToast('复制失败：未找到模块内容')
      return
    }

    try {
      // 临时添加到 body 以正确渲染
      const tempContainer = document.createElement('div')
      tempContainer.style.position = 'fixed'
      tempContainer.style.left = '-9999px'
      tempContainer.style.top = '0'
      tempContainer.style.width = '800px' // 固定宽度
      tempContainer.appendChild(el)
      document.body.appendChild(tempContainer)

      const dataUrl = await toPng(el, {
        quality: 0.95,
        backgroundColor: getComputedStyle(document.body).backgroundColor,
        width: 800,
        style: {
          padding: '20px',
        }
      })

      document.body.removeChild(tempContainer)

      // 转换为 blob 并复制到剪贴板
      const response = await fetch(dataUrl)
      const blob = await response.blob()

      await navigator.clipboard.write([
        new ClipboardItem({
          'image/png': blob
        })
      ])

      showToast('已复制为图片')
    } catch (err) {
      console.error('复制图片失败:', err)
      showToast('复制图片失败')
    }
    setIsOpen(false)
  }

  return (
    <>
      <span className="module-copy-wrapper">
        <button
          ref={buttonRef}
          className="module-copy-btn"
          title="复制模块"
          onClick={() => setIsOpen(!isOpen)}
        >
          📋
        </button>

        {isOpen && (
          <div ref={menuRef} className="module-copy-menu">
            <button className="module-copy-menu-item" onClick={copyMarkdown}>
              <span className="icon">📝</span>
              <span>Markdown</span>
            </button>
            <button className="module-copy-menu-item" onClick={copyPlainText}>
              <span className="icon">📄</span>
              <span>纯文本</span>
            </button>
            <button className="module-copy-menu-item" onClick={copyAsImage}>
              <span className="icon">🖼️</span>
              <span>图片</span>
            </button>
          </div>
        )}
      </span>

      {toast && <div className="module-copy-toast">{toast}</div>}
    </>
  )
}
