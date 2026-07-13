import React, { useState } from 'react'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { Copy, CheckCheck } from 'lucide-react'

// 代码块组件：带语言标签与复制按钮
function CodeBlock({ language, code }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch (err) {
      console.error('复制代码失败:', err)
    }
  }

  return (
    <div className="relative group my-3 rounded-lg overflow-hidden border border-border">
      <div className="flex items-center justify-between px-3 py-1.5 bg-bg-subtle border-b border-border">
        <span className="text-xs text-text-subtle font-mono">{language || 'code'}</span>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1 text-xs text-text-muted hover:text-text transition-colors"
          title="复制代码"
        >
          {copied ? <CheckCheck size={12} /> : <Copy size={12} />}
          {copied ? '已复制' : '复制'}
        </button>
      </div>
      <SyntaxHighlighter
        style={oneDark}
        language={language || 'text'}
        PreTag="div"
        customStyle={{
          margin: 0,
          borderRadius: 0,
          fontSize: '0.85rem',
          background: 'rgb(var(--bg-subtle))',
        }}
        codeTagProps={{
          style: {
            fontFamily:
              '"JetBrains Mono", "Fira Code", ui-monospace, SFMono-Regular, Menlo, monospace',
          },
        }}
        showLineNumbers={false}
      >
        {code}
      </SyntaxHighlighter>
    </div>
  )
}

export default CodeBlock
