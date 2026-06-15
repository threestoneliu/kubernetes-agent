export function ErrorToast({ message, onDismiss }: { message: string; onDismiss: () => void }) {
  return (
    <div className="toast" role="alert" onClick={onDismiss}>
      <span>{message}</span>
      <button
        style={{
          marginLeft: 12,
          background: 'transparent',
          border: 'none',
          color: 'white',
          cursor: 'pointer',
        }}
        onClick={(e) => {
          e.stopPropagation()
          onDismiss()
        }}
      >
        关闭
      </button>
    </div>
  )
}