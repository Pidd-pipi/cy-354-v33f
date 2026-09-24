<template>
  <div class="page">
    <h2>{{ isEdit ? '编辑商品' : '发布商品' }}</h2>
    <el-card style="max-width: 640px">
      <el-alert
        v-if="isEdit"
        type="info"
        :closable="false"
        title="仅在售商品可编辑；若保存时商品已在别处被修改，将拒绝保存并载入最新内容"
        style="margin-bottom: 12px"
      />
      <ProductForm v-if="!pageLoading" ref="formRef" :initial="initial" />
      <el-button type="primary" :loading="submitting" @click="submit">{{ isEdit ? '保存' : '发布' }}</el-button>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import ProductForm, { type ProductFormValue } from '../components/common/ProductForm.vue'
import { createProduct, getProduct, updateProduct } from '../api/product'
import { useAuthStore } from '../stores/authStore'
import { useRoute, useRouter } from 'vue-router'
import type { Product } from '../types'

const formRef = ref<InstanceType<typeof ProductForm>>()
const submitting = ref(false)
const pageLoading = ref(false)
const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()

const editId = computed(() => Number(route.query.edit) || 0)
const isEdit = computed(() => editId.value > 0)
const editingVersion = ref(0)
const initial = ref<Partial<ProductFormValue> | undefined>(undefined)

function toFormValue(p: Product): Partial<ProductFormValue> {
  return {
    title: p.title,
    description: p.description,
    price: p.price,
    category: p.category,
    condition: p.condition,
    campus: p.campus,
    trade_location: p.trade_location,
    images: p.images,
  }
}

onMounted(async () => {
  if (!isEdit.value) return
  if (!authStore.token) {
    ElMessage.warning('请先登录')
    router.push('/login')
    return
  }
  pageLoading.value = true
  try {
    const res = await getProduct(editId.value)
    const p = res.data
    if (p.seller_id !== authStore.user?.id) {
      ElMessage.error('只能编辑自己的商品')
      router.replace('/products')
      return
    }
    if (p.status !== 'on_sale') {
      ElMessage.warning('商品已售出或已下架，不可编辑')
      router.replace('/products')
      return
    }
    editingVersion.value = p.version
    initial.value = toFormValue(p)
  } finally {
    pageLoading.value = false
  }
})

async function submit() {
  if (!authStore.token) {
    ElMessage.warning('请先登录')
    router.push('/login')
    return
  }
  const form = formRef.value?.form
  if (!form || !form.title || !form.category || !form.campus || form.price <= 0) {
    ElMessage.warning('请填写完整信息')
    return
  }
  submitting.value = true
  try {
    if (isEdit.value) {
      await updateProduct(editId.value, {
        title: form.title,
        description: form.description,
        price: form.price,
        condition: form.condition,
        trade_location: form.trade_location,
        version: editingVersion.value,
      })
      ElMessage.success('保存成功')
    } else {
      await createProduct({ ...form })
      ElMessage.success('发布成功')
    }
    router.push('/products')
  } catch (err) {
    applyLatestOnConflict(err)
  } finally {
    submitting.value = false
  }
}

// A 409 carries the latest product in data: refresh the form with it instead
// of overwriting the other change, and let the user decide whether to retry.
function applyLatestOnConflict(err: unknown) {
  const resp = (err as { response?: { status?: number; data?: { data?: Product } } })?.response
  const latest = resp?.status === 409 ? resp.data?.data : undefined
  if (!latest) return
  editingVersion.value = latest.version
  initial.value = toFormValue(latest)
  ElMessage.warning('已载入最新内容，请确认后再保存')
}
</script>
