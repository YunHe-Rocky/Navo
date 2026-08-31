<script setup lang="ts">
import StateGlyph from "./components/StateGlyph.vue";
import OverviewPage from "./features/overview/OverviewPage.vue";
import ConnectionPage from "./features/capture/ConnectionPage.vue";
import SourcesPage from "./features/nodes/SourcesPage.vue";
import CorePage from "./features/core/CorePage.vue";
import TrafficPage from "./features/traffic/TrafficPage.vue";
import DiagnosticsPage from "./features/diagnostics/DiagnosticsPage.vue";
import SettingsPage from "./features/settings/SettingsPage.vue";
import { provideNavoApplication } from "./features/application/context";
import { useNavoApplication } from "./features/application/useNavoApplication";

const application = useNavoApplication();
provideNavoApplication(application);
const {
  navigation,
  navigationGroups,
  page,
  theme,
  dashboard,
  loading,
  notice,
  failure,
  routeRequired,
  activityVisible,
  activityLabel,
  activityProgress,
  dismissedFaultID,
  tunRetryButton,
  closeDialogOpen,
  closeAction,
  rememberCloseAction,
  closeActionBusy,
  closeActionError,
  closePrimaryButton,
  activeRoute,
  appState,
  pageTitle,
  currentNavigation,
  showTUNFault,
  setTheme,
  showCardFeedback,
  changePage,
  goToRouteSelection,
  setCapture,
  connectionLabel,
  performCloseAction,
  dismissCloseChoice,
} = application;
</script>

<template>
  <div class="app-shell" @pointerdown="showCardFeedback">
    <aside class="sidebar">
      <div class="brand">
        <StateGlyph :state="appState.icon" size="lg" label="Navo 当前连接状态" />
        <div><strong>Navo</strong><span>网络控制台</span></div>
      </div>
      <nav aria-label="主导航">
        <section v-for="group in navigationGroups" :key="group" class="nav-group" :aria-label="group">
          <h2>{{ group }}</h2>
          <button
            v-for="item in navigation.filter((entry) => entry.group === group)"
            :key="item.id"
            :class="{ active: page === item.id }"
            :aria-current="page === item.id ? 'page' : undefined"
            :aria-label="`${item.label}：${item.description}`"
            :title="item.description"
            @click="changePage(item.id)"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path :d="item.icon" /></svg>
            <span><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span>
          </button>
        </section>
      </nav>
      <div class="service-state">
        <StateGlyph :state="appState.icon" size="sm" />
        <div><strong :title="connectionLabel()">{{ connectionLabel() }}</strong><small>{{ activeRoute?.name || dashboard.core.core_id }}</small></div>
      </div>
    </aside>

    <main tabindex="0" aria-label="Navo 主内容，可使用方向键和滚轮滚动">
      <header>
        <div class="page-heading"><span>{{ currentNavigation?.group }}</span><h1>{{ pageTitle }}</h1><p>{{ currentNavigation?.description }}</p></div>
        <div class="theme-switch" role="group" aria-label="界面形态">
          <button :class="{ active: theme === 'day' }" :aria-pressed="theme === 'day'" @click="setTheme('day')">日</button>
          <button :class="{ active: theme === 'night' }" :aria-pressed="theme === 'night'" @click="setTheme('night')">夜</button>
        </div>
      </header>

      <div v-if="activityVisible" class="activity-progress" role="progressbar" aria-live="polite" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="activityProgress">
        <div><span>{{ activityLabel }}</span><b>{{ activityProgress }}%</b></div>
        <i><em :style="{ width: `${activityProgress}%` }"></em></i>
      </div>

      <div class="feedback" aria-live="polite">
        <section v-if="failure" class="feedback-panel error" role="alert">
          <div><strong>操作未完成</strong><p>{{ failure }}</p></div>
          <button v-if="routeRequired" class="feedback-action" @click="goToRouteSelection">前往连接管理</button>
        </section>
        <p v-else-if="notice" class="feedback-panel success" role="status">{{ notice }}</p>
      </div>

      <div
        v-if="showTUNFault"
        class="capture-modal-backdrop"
        role="presentation"
        @keydown.esc="dismissedFaultID = dashboard.capture.fault_id"
      >
        <section
          class="capture-modal"
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="tun-fault-title"
          aria-describedby="tun-fault-description"
        >
          <span class="eyebrow">TUN 运行异常</span>
          <h2 id="tun-fault-title">虚拟网卡已异常停止</h2>
          <p id="tun-fault-description">{{ dashboard.capture.last_error || "Navo 已停止内核并回滚网络配置，避免留下无效路由。" }}</p>
          <div class="capture-modal-actions">
            <button class="secondary" @click="dismissedFaultID = dashboard.capture.fault_id">关闭</button>
            <button ref="tunRetryButton" class="primary" :disabled="loading" @click="setCapture('tun')">重新启动 TUN</button>
          </div>
        </section>
      </div>

      <OverviewPage v-if="page === 'overview'" />

      <ConnectionPage v-else-if="page === 'connection'" />

      <SourcesPage v-else-if="page === 'sources'" />

      <CorePage v-else-if="page === 'cores'" />

      <TrafficPage v-else-if="page === 'traffic'" />

      <DiagnosticsPage v-else-if="page === 'ip'" />

      <SettingsPage v-else />
    </main>
  </div>

	<Teleport to="body">
		<div
			v-if="closeDialogOpen"
			class="close-dialog-backdrop"
			role="presentation"
			@keydown.esc="dismissCloseChoice"
		>
			<section
				class="close-dialog"
				role="dialog"
				aria-modal="true"
				aria-labelledby="close-dialog-title"
				aria-describedby="close-dialog-description"
			>
				<div class="close-dialog-heading">
					<StateGlyph state="default" size="md" label="Navo 关闭选项" />
					<div>
						<span class="eyebrow">窗口操作</span>
						<h2 id="close-dialog-title">关闭 Navo 窗口</h2>
					</div>
				</div>
				<p id="close-dialog-description">请选择最小化到系统托盘，或安全停止代理、恢复网络设置后彻底退出。</p>

				<fieldset class="close-action-options" :disabled="closeActionBusy">
					<legend>本次关闭操作</legend>
					<label :class="{ selected: closeAction === 'minimize' }">
						<input v-model="closeAction" type="radio" value="minimize" />
						<span><strong>最小化到托盘</strong><small>代理继续运行；图标可能位于任务栏右下角“^”折叠区。</small></span>
					</label>
					<label :class="{ selected: closeAction === 'exit' }">
						<input v-model="closeAction" type="radio" value="exit" />
						<span><strong>彻底退出程序</strong><small>执行安全关闭流程，并恢复 Navo 管理的网络状态。</small></span>
					</label>
				</fieldset>

				<label class="remember-close-action">
					<input v-model="rememberCloseAction" type="checkbox" :disabled="closeActionBusy" />
					<span><strong>记住本次选择</strong><small>仅在本次开机期间有效，最长 24 小时。</small></span>
				</label>
				<p v-if="closeActionError" class="close-dialog-error" role="alert">{{ closeActionError }}</p>
				<div class="close-dialog-actions">
					<button class="secondary" :disabled="closeActionBusy" @click="dismissCloseChoice">取消</button>
					<button
						ref="closePrimaryButton"
						:class="closeAction === 'exit' ? 'danger' : 'primary'"
						:disabled="closeActionBusy"
						@click="performCloseAction(closeAction, rememberCloseAction)"
					>
						{{ closeActionBusy ? (closeAction === "exit" ? "正在安全退出…" : "正在最小化…") : closeAction === "exit" ? "安全退出" : "最小化到托盘" }}
					</button>
				</div>
			</section>
		</div>
	</Teleport>
</template>
