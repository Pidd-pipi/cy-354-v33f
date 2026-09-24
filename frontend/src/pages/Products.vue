<template>
  <div class="page">
    <h2>商品广场</h2>
    <el-form inline class="filters">
      <el-form-item label="分类">
        <el-select v-model="query.category" clearable placeholder="全部分类" style="width: 160px">
          <el-option v-for="c in PRODUCT_CATEGORIES" :key="c.value" :label="c.label" :value="c.value" />
        </el-select>
      </el-form-item>
      <el-form-item label="校区">
        <el-input v-model="query.campus" placeholder="输入校区" style="width: 140px" clearable />
      </el-form-item>
      <el-form-item label="关键词">
        <el-input v-model="query.keyword" placeholder="搜索标题/描述" style="width: 180px" clearable />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="load">搜索</el-button>
      </el-form-item>
    </el-form>
    <el-row :gutter="16">
      <el-col v-for="p in products" :key="p.id" :span="6" class="col">
        <ProductCard
          :product="p"
          :can-edit="p.seller_id === authStore.user?.id"
          @detail="showDetail"
          @buy="buy"
          @chat="chat"
          @edit="openEdit"
        />
      </el-col>
    </el-row>
    <el-empty v-if="!loading && products.length === 0" description="暂无商品" />
    <el-dialog v-model="detailVisible" :title="current?.title" width="520px">
      <el-descriptions :column="2" border v-if="current">
        <el-descriptions-item label="分类">{{ categoryLabel(current.category) }}</el-descriptions-item>
        <el-descriptions-item label="成色">{{ current.condition }}</el-descriptions-item>
        <el-descriptions-item label="校区">{{ current.campus }}</el-descriptions-item>
        <el-descriptions-item label="交易地点">{{ current.trade_location }}</el-descriptions-item>
        <el-descriptions-item label="价格">¥{{ current.price.toFixed(2) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ productStatusLabel(current.status) }}</el-descriptions-item>
        <el-descriptions-item label="描述" :span="2">{{ current.description }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
    <el-dialog v-model="editVisible" title="编辑在售商品" width="520px">
      <ProductForm v-if="editing" ref="editFormRef" mode="edit" :product="editing" />
      <el-alert
        v-if="editConflict"
        :title="'内容已在别处被修改，已为你载入最新版本，请确认后重新保存'"
        type="warning"
        :closable="false"
        class="conflict-tip"
      />
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import ProductCard from '../components/common/ProductCard.vue'
import ProductForm from '../components/common/ProductForm.vue'
import { PRODUCT_CATEGORIES, categoryLabel, productStatusLabel } from '../constants/product'
import { useProducts } from '../hooks/useProducts'
import { createTradeOrder } from '../api/tradeOrder'
import { updateProduct } from '../api/product'
import { createConversation } from '../api/conversation'
import type { Product } from '../types'
import { useAuthStore } from '../stores/authStore'
import { useRouter } from 'vue-router'

const { products, loading, load } = useProducts()
const query = reactive<{ category?: string; campus?: string; keyword?: string }>({})
const detailVisible = ref(false)
const current = ref<Product | null>(null)
const authStore = useAuthStore()
const router = useRouter()

const editVisible = ref(false)
const editing = ref<Product | null>(null)
const editFormRef = ref<InstanceType<typeof ProductForm>>()
const saving = ref(false)
const editConflict = ref(false)
// Revision observed when the edit dialog was opened; echoed unchanged on retry.
const openedRevision = ref(0)

function showDetail(p: Product) {
  current.value = p
  detailVisible.value = true
}

function openEdit(p: Product) {
  editing.value = { ...p }
  openedRevision.value = p.revision
  editConflict.value = false
  editVisible.value = true
}

function replaceInList(p: Product) {
  const idx = products.value.findIndex((item) => item.id === p.id)
  if (idx >= 0) products.value[idx] = p
  if (current.value?.id === p.id) current.value = p
}

async function saveEdit() {
  const target = editing.value
  const form = editFormRef.value?.form
  if (!target || !form) return
  if (!form.title || !form.condition || !form.trade_location || form.price <= 0) {
    ElMessage.warning('请填写完整信息')
    return
  }
  saving.value = true
  try {
    const res = await updateProduct(target.id, {
      title: form.title,
      description: form.description,
      price: form.price,
      condition: form.condition,
      trade_location: form.trade_location,
      revision: openedRevision.value,
    })
    replaceInList(res.data)
    editing.value = res.data
    openedRevision.value = res.data.revision
    editVisible.value = false
    ElMessage.success('已保存')
  } catch (e) {
    // 409: another save landed first, or the product was sold/taken down.
    // The backend refuses the overwrite and returns the newest version in data.
    if (axios.isAxiosError(e) && e.response?.status === 409) {
      const latest = e.response.data?.data as Product | undefined
      if (latest) {
        editing.value = latest
        openedRevision.value = latest.revision
        replaceInList(latest)
        editConflict.value = true
      } else {
        ElMessage.error(e.response.data?.message || '保存失败')
        editVisible.value = false
        await load()
      }
    }
  } finally {
    saving.value = false
  }
}

async function buy(p: Product) {
  if (!authStore.token) {
    ElMessage.warning('请先登录')
    router.push('/login')
    return
  }
  await createTradeOrder(p.id)
  ElMessage.success('已下单，等待卖家确认')
}

async function chat(p: Product) {
  if (!authStore.token) {
    ElMessage.warning('请先登录')
    router.push('/login')
    return
  }
  await createConversation(p.id)
  ElMessage.success('已发起私信')
  router.push('/messages')
}

onMounted(() => load())
</script>

<style scoped>
.filters {
  margin-bottom: 8px;
}
.col {
  margin-bottom: 16px;
}
.conflict-tip {
  margin-top: 8px;
}
</style>
