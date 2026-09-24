<template>
  <el-form :model="form" label-width="90px">
    <el-form-item label="标题">
      <el-input v-model="form.title" placeholder="商品标题" maxlength="64" />
    </el-form-item>
    <el-form-item label="描述">
      <el-input v-model="form.description" type="textarea" :rows="3" placeholder="描述商品" />
    </el-form-item>
    <el-form-item label="价格">
      <el-input-number v-model="form.price" :min="0.01" :precision="2" />
    </el-form-item>
    <el-form-item v-if="mode === 'create'" label="分类">
      <el-select v-model="form.category" placeholder="选择分类" style="width: 100%">
        <el-option v-for="c in PRODUCT_CATEGORIES" :key="c.value" :label="c.label" :value="c.value" />
      </el-select>
    </el-form-item>
    <el-form-item label="成色">
      <el-input v-model="form.condition" placeholder="如：全新/九成新" maxlength="16" />
    </el-form-item>
    <el-form-item v-if="mode === 'create'" label="校区">
      <el-input v-model="form.campus" placeholder="如：东校区" />
    </el-form-item>
    <el-form-item label="交易地点">
      <el-input v-model="form.trade_location" placeholder="如：图书馆门口" maxlength="128" />
    </el-form-item>
    <el-form-item v-if="mode === 'create'" label="实拍图URL">
      <el-input v-model="form.images" placeholder="可选，多个用逗号分隔" />
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { reactive, watch } from 'vue'
import { PRODUCT_CATEGORIES } from '../../constants/product'
import type { Product } from '../../types'

export interface ProductFormValue {
  title: string
  description: string
  price: number
  category: string
  condition: string
  campus: string
  trade_location: string
  images: string
}

const props = withDefaults(
  defineProps<{ mode?: 'create' | 'edit'; product?: Product | null }>(),
  { mode: 'create', product: null },
)

const form = reactive<ProductFormValue>({
  title: '',
  description: '',
  price: 0,
  category: '',
  condition: '',
  campus: '',
  trade_location: '',
  images: '',
})

function fillFrom(p: Product | null) {
  form.title = p?.title ?? ''
  form.description = p?.description ?? ''
  form.price = p?.price ?? 0
  form.category = p?.category ?? ''
  form.condition = p?.condition ?? ''
  form.campus = p?.campus ?? ''
  form.trade_location = p?.trade_location ?? ''
  form.images = p?.images ?? ''
}

watch(() => props.product, fillFrom, { immediate: true })

defineExpose({ form })
</script>
