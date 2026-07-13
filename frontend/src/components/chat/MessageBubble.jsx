import React, { useState, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Copy, CheckCheck, Bot, User } from 'lucide-react'
import CodeBlock from './CodeBlock'
import { fixStreamingMarkdown } from '../../hooks/useChat'

// Markdown 渲染器的统一组件映射
const markdownComponents = {
  code({ className, children, ...props }) {
    const match = /language-(\w+)/.exec(className || '')
    const codeStr = String(children).replace(/\n$/, '')
    if (match) {
      return <CodeBlock language={match[1]} code={codeStr} />
    }
    return (
      <code
        className="px-1 py-0.5 rounded bg-bg-subtle text-primary-600 dark:text-primary-300 font-mono text-[0.85em]"
        {...props}
      >
        {children}
      </code>
    )
  },
  p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed">{children}</p>,
  h1: ({ children }) => (
    <h1 className="text-xl font-bold mb-3 mt-4 first:mt-0">{children}</h1>
  ),
  h2: ({ children }) => (
    <h2 className="text-lg font-bold mb-2 mt-4 first:mt-0">{children}</h2>
  ),
  h3: ({ children }) => (
    <h3 className="text-base font-bold mb-2 mt-3 first:mt-0">{children}</h3>
  ),
  ul: ({ children }) => (
    <ul className="list-disc pl-5 mb-2 space-y-1">{children}</ul>
  ),
  ol: ({ children }) => (
    <ol className="list-decimal pl-5 mb-2 space-y-1">{children}</ol>
  ),
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="border-l-4 border-primary-400 pl-4 italic text-text-muted my-2">
      {children}
    </blockquote>
  ),
  a: ({ href, children }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-primary-500 hover:text-primary-400 hover:underline"
    >
      {children}
    </a>
  ),
  table: ({ children }) => (
    <div className="overflow-x-auto my-3">
      <table className="border-collapse w-full text-sm">{children}</table>
    </div>
  ),
  th: ({ children }) => (
    <th className="border border-border px-3 py-2 bg-bg-subtle text-left">
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td className="border border-border px-3 py-2">{children}</td>
  ),
  hr: () => <hr className="my-4 border-border" />,
  strong: ({ children }) => <strong className="font-bold">{children}</strong>,
  em: ({ children }) => <em className="italic">{children}</em>,
  pre: ({ children }) => <div className="my-1">{children}</div>,
}

// 单条消息气泡
function MessageBubble({ msg, isStreamingMsg }) {
  const [copied, setCopied] = useState(false)
  const isUser = msg.role === 'user'

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(msg.content)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch (err) {
      console.error('复制失败:', err)
    }
  }, [msg.content])

  const content = isStreamingMsg
    ? fixStreamingMarkdown(msg.content)
    : msg.content

  return (
    <div className={`flex gap-3 ${isUser ? 'flex-row-reverse' : ''}`}>
      <div
        className={`w-9 h-9 rounded-full flex-shrink-0 flex items-center justify-center ${
          isUser
            ? 'bg-primary-500'
            : 'bg-gradient-to-br from-primary-500 to-accent-500'
        }`}
      >
        {isUser ? (
          <User className="w-5 h-5 text-white" />
        ) : (
          <Bot className="w-5 h-5 text-white" />
        )}
      </div>
      <div className={`flex-1 max-w-[85%] ${isUser ? 'text-right' : ''}`}>
        <div
          className={`inline-block text-left ${
            isUser
              ? 'bg-primary-500 text-white rounded-2xl rounded-tr-md px-4 py-2.5'
              : 'bg-surface rounded-2xl rounded-tl-md px-4 py-3 border border-border'
          }`}
        >
          {isUser ? (
            <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
              {msg.content}
            </p>
          ) : (
            <div className="markdown-body break-words">
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={markdownComponents}
              >
                {content}
              </ReactMarkdown>
              {isStreamingMsg && (
                <span className="inline-block w-1.5 h-4 ml-1 bg-primary-400 animate-pulse align-middle rounded" />
              )}
            </div>
          )}
        </div>
        {!isUser && !isStreamingMsg && (
          <div className="mt-1.5 flex items-center gap-3 text-xs text-text-subtle opacity-0 hover:opacity-100 transition-opacity pl-1">
            <button
              onClick={handleCopy}
              className="hover:text-text-muted flex items-center gap-1"
            >
              {copied ? <CheckCheck size={12} /> : <Copy size={12} />}
              {copied ? '已复制' : '复制'}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

export default MessageBubble
