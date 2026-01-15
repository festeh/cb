import { ref, computed, watch } from 'vue'
import { useLogger } from '../utils/logger'

const log = useLogger('Images')

export function useImages() {
  const images = ref([])
  const currentIndex = ref(0)

  const currentImage = computed(() => images.value[currentIndex.value] || null)
  const currentImageUrl = computed(() =>
    currentImage.value ? '/api/images/' + currentImage.value : ''
  )
  const canGoBack = computed(() => currentIndex.value < images.value.length - 1)
  const canGoForward = computed(() => currentIndex.value > 0)

  async function loadImages() {
    try {
      const resp = await fetch('/api/images')
      images.value = await resp.json()
      log.info(`Loaded ${images.value.length} images`)
    } catch (err) {
      log.error('Failed to load images', err)
    }
  }

  async function loadCurrentImage() {
    try {
      const resp = await fetch('/api/current-image')
      const data = await resp.json()
      if (data.image && images.value.length > 0) {
        const idx = images.value.indexOf(data.image)
        if (idx >= 0) {
          currentIndex.value = idx
          log.info(`Current image: ${data.image}`)
        }
      }
    } catch (err) {
      log.error('Failed to load current image', err)
    }
  }

  async function reportCurrentImage() {
    const img = currentImage.value
    if (!img) return
    try {
      await fetch('/api/current-image', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ image: img })
      })
    } catch (err) {
      log.error('Failed to report current image', err)
    }
  }

  function goBack() {
    if (canGoBack.value) {
      currentIndex.value++
    }
  }

  function goForward() {
    if (canGoForward.value) {
      currentIndex.value--
    }
  }

  function addImage(filename) {
    log.info(`New image: ${filename}`)
    images.value.unshift(filename)
    currentIndex.value = 0
  }

  // Report image selection to backend when it changes
  watch(currentIndex, reportCurrentImage)

  return {
    images,
    currentIndex,
    currentImage,
    currentImageUrl,
    canGoBack,
    canGoForward,
    loadImages,
    loadCurrentImage,
    goBack,
    goForward,
    addImage
  }
}
