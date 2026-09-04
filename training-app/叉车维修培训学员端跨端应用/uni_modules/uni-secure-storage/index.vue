<template>
  <view class="page">
    <text class="title">安全存储演示页（uni-secure-storage）</text>
    <text class="desc">真机验收闭环：写入 → 读取一致 → 重启仍在 → 删除 → has=false</text>

    <view class="row">
      <text class="label">能力查询</text>
      <button class="btn" @click="onQueryCaps">查询</button>
    </view>
    <text class="result">支持: {{ capsText }}</text>
    <text class="result">后端: {{ backendText }}</text>

    <view class="row">
      <text class="label">Key</text>
      <input class="input" v-model="demoKey" placeholder="条目名" />
    </view>
    <view class="row">
      <text class="label">Value</text>
      <input class="input" v-model="demoValue" placeholder="要保存的内容" />
    </view>

    <view class="btn-row">
      <button class="btn" @click="onWrite">写入</button>
      <button class="btn" @click="onRead">读取</button>
      <button class="btn" @click="onHas">存在?</button>
      <button class="btn" @click="onRemove">删除</button>
      <button class="btn" @click="onClear">清空</button>
    </view>

    <text class="result">{{ resultText }}</text>
  </view>
</template>

<script>
  import {
    setSecureItem,
    getSecureItem,
    hasSecureItem,
    removeSecureItem,
    clearSecureItems,
    getSecureStorageCapabilities,
  } from '@/uni_modules/uni-secure-storage'

  export default {
    data() {
      return {
        demoKey: 'demo_token',
        demoValue: 'secret-value-123',
        capsText: '?',
        backendText: '?',
        resultText: '',
      }
    },
    methods: {
      onQueryCaps() {
        const caps = getSecureStorageCapabilities()
        this.capsText = caps.supported ? '是' : '否'
        this.backendText = caps.backend
      },
      onWrite() {
        const r = setSecureItem(this.demoKey, this.demoValue)
        this.resultText = r.ok ? '写入成功' : `写入失败: ${r.errMsg}`
      },
      onRead() {
        const r = getSecureItem(this.demoKey)
        if (!r.ok) {
          this.resultText = `读取失败: ${r.errMsg}`
        } else if (!r.found) {
          this.resultText = '条目不存在'
        } else {
          this.resultText = `读取成功: ${r.value}`
        }
      },
      onHas() {
        const r = hasSecureItem(this.demoKey)
        this.resultText = r.ok ? (r.exists ? '存在' : '不存在') : `查询失败: ${r.errMsg}`
      },
      onRemove() {
        const r = removeSecureItem(this.demoKey)
        this.resultText = r.ok ? '删除成功' : `删除失败: ${r.errMsg}`
      },
      onClear() {
        const r = clearSecureItems()
        this.resultText = r.ok ? '清空成功' : `清空失败: ${r.errMsg}`
      },
    },
  }
</script>

<style>
  .page {
    padding: 24rpx;
  }
  .title {
    font-size: 36rpx;
    font-weight: bold;
    margin-bottom: 12rpx;
  }
  .desc {
    font-size: 24rpx;
    color: #888888;
    margin-bottom: 24rpx;
  }
  .row {
    flex-direction: row;
    align-items: center;
    margin-bottom: 16rpx;
  }
  .label {
    width: 140rpx;
    font-size: 28rpx;
  }
  .input {
    flex: 1;
    border: 1rpx solid #dddddd;
    padding: 12rpx;
    font-size: 28rpx;
  }
  .btn-row {
    flex-direction: row;
    margin-bottom: 16rpx;
  }
  .btn {
    margin-right: 16rpx;
  }
  .result {
    font-size: 28rpx;
    color: #0066cc;
    margin-bottom: 12rpx;
  }
</style>
