import { ref } from 'vue'

/** markdown 富文本里图片点击放大(图册模式)。挂在内容容器上使用:
 *  const { visible, urlList, initialIndex, onContainerClick, close } = useImagePreview()
 *  <div class="md-body" v-html="..." @click="onContainerClick" />
 *  <el-image-viewer v-if="visible" :url-list="urlList" :initial-index="initialIndex" @close="close" />
 */
export function useImagePreview() {
  const visible = ref(false)
  const urlList = ref<string[]>([])
  const initialIndex = ref(0)

  function onContainerClick(e: MouseEvent) {
    const img = (e.target as HTMLElement).closest<HTMLImageElement>('img[src]')
    if (!img) return
    e.preventDefault()
    const container = e.currentTarget as HTMLElement
    const imgs = Array.from(container.querySelectorAll<HTMLImageElement>('img[src]'))
    const idx = imgs.indexOf(img)
    urlList.value = imgs.map(i => i.currentSrc || i.src)
    initialIndex.value = Math.max(0, idx)
    visible.value = true
  }

  function close() {
    visible.value = false
  }

  return { visible, urlList, initialIndex, onContainerClick, close }
}
