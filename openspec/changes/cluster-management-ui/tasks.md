## 1. UI Changes

- [x] 1.1 Add `showModal` state to `ClusterView` (boolean, default `false`)
- [x] 1.2 Replace page-top embedded form with toolbar: `[已配置的集群 (N个)] [刷新] [新建集群]`
- [x] 1.3 Implement "新建集群" button that sets `showModal = true`
- [x] 1.4 Add `<Modal>` wrapping the existing form (name + kubeconfig inputs + submit/cancel)
- [x] 1.5 Wire form submit: on success `setShowModal(false)`, on cancel `setShowModal(false)`
- [x] 1.6 Remove the embedded `<form>` block from the page body
