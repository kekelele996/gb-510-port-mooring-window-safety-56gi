
<script setup lang="ts">defineProps<{ modelValue: boolean; title: string }>(); const emit = defineEmits<{ 'update:modelValue': [value: boolean]; confirm: [] }>();</script>
<template><el-dialog :model-value="modelValue" :title="title" width="440px" @close="emit('update:modelValue', false)"><slot/><template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" @click="emit('confirm')">确认</el-button></template></el-dialog></template>
