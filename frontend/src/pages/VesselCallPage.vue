
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useVesselCallStore } from '../stores/vessel-call'; const store = useVesselCallStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[0]" :store="store"/></template>
