<template>
  <div class="page">
    <h2>我的交易</h2>
    <el-card v-for="o in orders" :key="o.id" class="order-card">
      <div class="order-row">
        <div>
          <TradeStatusBadge :status="o.status" />
          <span class="order-id">订单 #{{ o.id }} · {{ productMap[o.product_id]?.title ?? `商品 #${o.product_id}` }}</span>
          <el-tag
            v-if="productMap[o.product_id]"
            :type="productStatusType(productMap[o.product_id].status) as any"
            size="small"
            class="product-status"
          >{{ productStatusLabel(productMap[o.product_id].status) }}</el-tag>
          <p class="order-meta">
            买家 #{{ o.buyer_id }} / 卖家 #{{ o.seller_id }} · {{ formatDateTime(o.created_at) }}
          </p>
          <p class="order-price">
            成交价（下单时冻结）：<span class="price">¥{{ o.price.toFixed(2) }}</span>
            <span v-if="priceChanged(o)" class="price-hint">
              商品当前售价 ¥{{ productMap[o.product_id].price.toFixed(2) }}，本订单不受影响
            </span>
          </p>
        </div>
        <div class="order-actions">
          <el-button v-if="o.status === 'pending' && o.buyer_id === authStore.user?.id" size="small" type="primary" @click="buyerConfirm(o.id)">确认收货</el-button>
          <el-button v-if="o.status === 'confirmed' && o.seller_id === authStore.user?.id" size="small" type="success" @click="sellerConfirm(o.id)">确认收款</el-button>
          <el-button v-if="o.status === 'pending'" size="small" type="danger" @click="cancel(o.id)">取消</el-button>
          <el-button v-if="o.status === 'completed'" size="small" @click="reviewDialog(o)">评价</el-button>
        </div>
      </div>
    </el-card>
    <el-empty v-if="orders.length === 0" description="暂无交易" />
    <el-dialog v-model="reviewVisible" title="信誉评价" width="420px">
      <el-form label-width="70px">
        <el-form-item label="评价">
          <el-select v-model="reviewForm.rating" style="width: 100%">
            <el-option v-for="r in REVIEW_RATINGS" :key="r.value" :label="r.label" :value="r.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="reviewForm.content" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewVisible = false">取消</el-button>
        <el-button type="primary" @click="submitReview">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import TradeStatusBadge from '../components/common/TradeStatusBadge.vue'
import { storeToRefs } from 'pinia'
import { useTradeStore } from '../stores/tradeStore'
import { useAuthStore } from '../stores/authStore'
import { buyerConfirm as buyerConfirmApi, sellerConfirm as sellerConfirmApi, cancelTradeOrder as cancelApi } from '../api/tradeOrder'
import { getProduct } from '../api/product'
import { createReview } from '../api/review'
import { REVIEW_RATINGS } from '../constants/trade'
import { productStatusLabel, productStatusType } from '../constants/product'
import { formatDateTime } from '../utils/dateFormat'
import type { Product, TradeOrder } from '../types'

const tradeStore = useTradeStore()
const { orders } = storeToRefs(tradeStore)
const { fetch } = tradeStore
const authStore = useAuthStore()
const reviewVisible = ref(false)
const reviewForm = reactive({ trade_id: 0, rating: 'good', content: '' })

// Snapshot of product rows keyed by id, used to show the current title/status
// next to the order. The order price itself is frozen and never refetched.
const productMap = ref<Record<number, Product>>({})

async function loadProducts() {
  const ids = [...new Set(orders.value.map((o) => o.product_id))]
  const entries = await Promise.all(
    ids.map(async (id) => {
      try {
        const res = await getProduct(id)
        return [id, res.data] as const
      } catch {
        return null
      }
    }),
  )
  productMap.value = Object.fromEntries(entries.filter(Boolean) as [number, Product][])
}

function priceChanged(o: TradeOrder): boolean {
  const p = productMap.value[o.product_id]
  return !!p && Math.abs(p.price - o.price) > 0.001
}

async function refresh() {
  await fetch()
  await loadProducts()
}

async function buyerConfirm(id: number) {
  await buyerConfirmApi(id)
  ElMessage.success('已确认收货')
  await refresh()
}

async function sellerConfirm(id: number) {
  await sellerConfirmApi(id)
  ElMessage.success('交易完成')
  await refresh()
}

async function cancel(id: number) {
  await cancelApi(id)
  ElMessage.success('已取消')
  await refresh()
}

function reviewDialog(o: TradeOrder) {
  reviewForm.trade_id = o.id
  reviewForm.rating = 'good'
  reviewForm.content = ''
  reviewVisible.value = true
}

async function submitReview() {
  await createReview({ trade_id: reviewForm.trade_id, rating: reviewForm.rating, content: reviewForm.content })
  ElMessage.success('评价成功')
  reviewVisible.value = false
}

onMounted(refresh)
</script>

<style scoped>
.order-card {
  margin-bottom: 12px;
}
.order-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.order-id {
  margin-left: 8px;
  font-size: 13px;
  color: #606266;
}
.order-meta {
  color: #909399;
  font-size: 12px;
  margin: 6px 0 0;
}
.product-status {
  margin-left: 8px;
}
.order-price {
  margin: 6px 0 0;
  font-size: 13px;
  color: #606266;
}
.order-price .price {
  color: #f56c6c;
  font-weight: 700;
}
.price-hint {
  margin-left: 8px;
  color: #909399;
  font-size: 12px;
}
</style>
