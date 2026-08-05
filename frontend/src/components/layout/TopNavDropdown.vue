<template>
  <div class="relative shrink-0" ref="rootRef">
    <button
      class="flex items-center gap-1 whitespace-nowrap text-sm font-medium px-2.5 py-1.5 rounded-md transition-colors"
      :class="anyActive
        ? 'text-primary-600 bg-primary-50 dark:bg-primary-500/10 dark:text-primary-400'
        : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-dark-800'"
      :aria-label="label"
      @click="open = !open"
    >
      {{ label }}
      <Icon name="chevronDown" size="sm" class="transition-transform duration-200" :class="{ 'rotate-180': open }" />
    </button>
    <transition name="dropdown">
      <div v-if="open" class="dropdown left-0 mt-2 w-52">
        <div class="py-1">
          <router-link
            v-for="item in items"
            :key="item.path"
            :to="item.path"
            class="dropdown-item"
            :class="{ 'bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-400': isActive(item.path) }"
            @click="close(); emit('select', item.path)"
          >
            <span v-if="item.iconSvg" class="h-4 w-4 flex-shrink-0 topnav-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
            <component v-else :is="item.icon" class="h-4 w-4 flex-shrink-0" />
            {{ item.label }}
          </router-link>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeSvg } from '@/utils/sanitize'
import type { NavItem } from './useUserNavItems'

const props = defineProps<{
  label: string
  items: NavItem[]
}>()

const emit = defineEmits<{
  select: [path: string]
}>()

const route = useRoute()
const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

const anyActive = computed(() => props.items.some((item) => isActive(item.path)))

function close() {
  open.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (rootRef.value && !rootRef.value.contains(event.target as Node)) {
    close()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}

/* Custom SVG icon: constrain size without overriding uploaded SVG colors */
.topnav-svg-icon {
  color: currentColor;
}

.topnav-svg-icon :deep(svg) {
  display: block;
  width: 1rem;
  height: 1rem;
}
</style>
