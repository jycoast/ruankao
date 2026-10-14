<script setup lang="ts">
import { renderMd, renderMdInline } from "@/utils/markdown";
import { ref, computed, onMounted, watch } from "vue";
import { useRouter } from "vue-router";
import { useUserStore } from "@/stores/user";
import { useSubjectStore } from "@/stores/subject";
import { questionApi, practiceApi } from "@/api";
import { ElMessage } from "element-plus";
import AnswerCard from "@/components/AnswerCard.vue";
import ShortcutHint from "@/components/ShortcutHint.vue";
import QuestionNoteEditor from "@/components/QuestionNoteEditor.vue";
import AIExplain from "@/components/AIExplain.vue";
import AIGradeCase from "@/components/AIGradeCase.vue";
import { useQuizShortcuts } from "@/composables/useQuizShortcuts";
import type { Question } from "@/types";
import {
  groupOptionsByBlank,
  annotateBlanksInStem,
  replaceBlankMarkers,
  extractTargetBlankNumber,
  correctLabelsByBlank,
  correctAnswerFromOptions,
  stemCoversTitle,
} from '@/utils/blanks';

const router = useRouter();
const userStore = useUserStore();
const subjectStore = useSubjectStore();

const categories = ref<any[]>([]);
const progress = ref<any[]>([]);
const activeType = ref(1);
const expandedMainCats = ref<Set<string>>(new Set());
const loading = ref(false);

const mode = ref<"list" | "practice">("list");
const questions = ref<Question[]>([]);
const currentIndex = ref(0);
const answers = ref<Record<number, string>>({});
// sessionAnswers 是「本轮已作答」的全量记录（题目 id → 完整答案），
// 与 answers 不同：后者只装当前这道题的各个空。答题卡和「继续刷题」都读它。
const sessionAnswers = ref<Record<number, string>>({});
const currentBlank = ref(0);
const showResult = ref(false);
const questionStartTime = ref(Date.now());
const result = ref<any>({});
const currentInput = ref("");
const currentKP = ref("");

// Start dialog
const showStartDialog = ref(false);
const startKP = ref("");
const startTotal = ref(0);
const startDone = ref(0);
const startResumable = ref(false);
const starting = ref(false);

const typeLabels: Record<number, string> = {
  1: "选择题",
  3: "案例分析",
  4: "论文",
};
const typeIcons: Record<number, string> = {
  1: "Edit",
  3: "Notebook",
  4: "Document",
};

async function loadCategories() {
  loading.value = true;
  try {
    const results = await Promise.all([
      questionApi.getCategories({ subject_id: subjectStore.currentSubjectId }),
      userStore.isLoggedIn
        ? practiceApi.getCategoryProgress().catch(() => ({ data: [] }))
        : Promise.resolve({ data: [] }),
    ]);
    const catRes = results[0] as any;
    const progRes = results[1] as any;
    categories.value = catRes.data || [];
    progress.value = progRes.data || [];
  } catch {}
  loading.value = false;
}

onMounted(loadCategories);

watch(
  () => subjectStore.currentSubjectId,
  () => {
    if (mode.value === "list") {
      loadCategories();
    }
  },
);

const progressMap = computed(() => {
  const map: Record<string, { done: number; correct: number; total: number }> =
    {};
  for (const p of progress.value) {
    map[p.knowledge_point] = {
      done: p.done,
      correct: p.correct,
      total: p.total,
    };
  }
  return map;
});

const filteredCategories = computed(() => {
  const grouped: Record<
    string,
    { main_cat: string; subs: { sub_cat: string; count: number }[] }
  > = {};
  for (const cat of categories.value) {
    if (cat.question_type !== activeType.value) continue;
    if (!grouped[cat.main_cat])
      grouped[cat.main_cat] = { main_cat: cat.main_cat, subs: [] };
    grouped[cat.main_cat].subs.push({ sub_cat: cat.sub_cat, count: cat.count });
  }
  return Object.values(grouped).sort((a, b) => {
    return (
      b.subs.reduce((s, x) => s + x.count, 0) -
      a.subs.reduce((s, x) => s + x.count, 0)
    );
  });
});

function mainCatTotal(subs: any[]) {
  return subs.reduce((s, x) => s + x.count, 0);
}

function mainCatDone(subs: any[]) {
  return subs.reduce((s, x) => {
    const p = progressMap.value[x.sub_cat];
    return s + (p ? p.done : 0);
  }, 0);
}

function toggleMainCat(cat: string) {
  if (expandedMainCats.value.has(cat)) {
    expandedMainCats.value.delete(cat);
  } else {
    expandedMainCats.value.add(cat);
  }
}

// currentQType 当前 tab 对应的题型，抽题与草稿定位都用它。
const currentQType = computed(() =>
  activeType.value === 4 ? 4 : activeType.value === 3 ? 3 : 1,
);

async function openStartDialog(kp: string, total: number) {
  if (!userStore.isLoggedIn) {
    router.push("/login");
    return;
  }
  const p = progressMap.value[kp];
  startKP.value = kp;
  startTotal.value = total;
  startDone.value = p ? p.done : 0;
  startResumable.value = false;
  showStartDialog.value = true;
  // 能否「继续」取决于有没有上次留下的答题卡，而不是这个知识点做过多少题
  try {
    const res: any = await practiceApi.sessionState({
      subject_id: subjectStore.currentSubjectId,
      question_type: currentQType.value,
      knowledge_point: kp,
    });
    if (startKP.value === kp) startResumable.value = !!res.data?.resumable;
  } catch {}
}

async function startPractice(fresh: boolean) {
  if (starting.value) return;
  starting.value = true;
  try {
    const res: any = await practiceApi.startPractice({
      subject_id: subjectStore.currentSubjectId,
      question_type: currentQType.value,
      knowledge_point: startKP.value,
      fresh,
    });
    const data = res.data || {};
    questions.value = data.questions || [];
    if (questions.value.length === 0) {
      ElMessage.warning("该知识点暂无题目");
      return;
    }
    // 后端的 answers 以题目 id 的字符串形式为键，转回数字键好和 question.id 直接对上
    sessionAnswers.value = {};
    for (const [qid, ans] of Object.entries(data.answers || {})) {
      sessionAnswers.value[Number(qid)] = ans as string;
    }
    currentKP.value = startKP.value;
    currentIndex.value = data.index || 0;
    showResult.value = false;
    result.value = {};
    questionStartTime.value = Date.now();
    applyAnswer(questions.value[currentIndex.value] || null);
    mode.value = "practice";
    showStartDialog.value = false;
    if (data.resumed) {
      ElMessage.success(
        `已恢复上次进度，从第 ${currentIndex.value + 1} 题继续`,
      );
    }
  } catch {
  } finally {
    starting.value = false;
  }
}

function backToList() {
  mode.value = "list";
  questions.value = [];
  // Reload progress
  if (userStore.isLoggedIn) {
    practiceApi
      .getCategoryProgress()
      .then((res: any) => {
        progress.value = res.data || [];
      })
      .catch(() => {});
  }
}

const currentQuestion = computed(() => {
  if (
    questions.value.length > 0 &&
    currentIndex.value < questions.value.length
  ) {
    return questions.value[currentIndex.value];
  }
  return null;
});

const blankGroups = computed(() => groupOptionsByBlank(currentQuestion.value?.options || []));

const totalBlanks = computed(() => blankGroups.value.length || 1);
const currentOptions = computed(
  () => blankGroups.value[currentBlank.value] || [],
);
const selectedAnswer = computed(() => answers.value[currentBlank.value] || "");

const targetBlankNumber = computed(() => {
  const q = currentQuestion.value;
  if (!q) return null;
  return extractTargetBlankNumber(q.content || q.title || '');
});
const activeBlankNo = computed(() => targetBlankNumber.value || (currentBlank.value + 1));
const correctLabels = computed(() => correctLabelsByBlank(currentQuestion.value?.options || []));
const renderedStem = computed(() => {
  const q = currentQuestion.value;
  if (!q) return '';
  const raw = q.content || q.title || '';
  const marked = annotateBlanksInStem(raw, currentBlank.value, totalBlanks.value || 1, answers.value);
  return replaceBlankMarkers(renderMd(marked));
});

const fullAnswer = computed(() => {
  let ans = "";
  for (let i = 0; i < totalBlanks.value; i++) {
    ans += answers.value[i] || "";
  }
  return ans;
});

const canSubmit = computed(() => {
  if (currentOptions.value.length === 0) return !!currentInput.value.trim();
  if (totalBlanks.value <= 1) return !!selectedAnswer.value;
  return blankGroups.value.every((_, i) => answers.value[i]);
});

const correctAnswer = computed(() => {
  return correctLabels.value.join('') || correctAnswerFromOptions(currentQuestion.value?.options || []);
});

function selectOption(label: string) {
  if (showResult.value) return;
  answers.value[currentBlank.value] = label;
}

async function submitAnswer() {
  if (!userStore.isLoggedIn) {
    router.push("/login");
    return;
  }
  const q = currentQuestion.value;
  if (!q) return;
  const ans =
    currentOptions.value.length === 0
      ? currentInput.value.trim()
      : fullAnswer.value;
  if (!ans) return;
  const res: any = await practiceApi.submitAnswer({
    question_id: q.id,
    user_answer: ans,
    practice_type: 5,
    time_spent: Math.floor((Date.now() - questionStartTime.value) / 1000),
    subject_id: subjectStore.currentSubjectId,
    question_type: currentQType.value,
    knowledge_point: currentKP.value,
  });
  sessionAnswers.value[q.id] = ans;
  result.value = res.data;
  showResult.value = true;
}

// applyAnswer 把某题已有的作答铺回作答区，供切题/续做时还原。
// 多空题按空位拆回每个空，主观题回填输入框。
function applyAnswer(q: Question | null) {
  answers.value = {};
  currentInput.value = "";
  currentBlank.value = 0;
  const saved = q ? sessionAnswers.value[q.id] : undefined;
  if (!saved) return;
  if (!q?.options || q.options.length === 0) {
    currentInput.value = saved;
    return;
  }
  for (let i = 0; i < saved.length; i++) {
    answers.value[i] = saved[i];
  }
}

function nextQuestion() {
  questionStartTime.value = Date.now();
  currentIndex.value++;
  showResult.value = false;
  result.value = {};
  applyAnswer(currentQuestion.value);
}

function isCurrentBlankCorrect(label: string) {
  return correctLabels.value[currentBlank.value] === label;
}

const answeredMap = computed(() => {
  const map: Record<number, boolean> = {};
  for (let i = 0; i < questions.value.length; i++) {
    const q = questions.value[i];
    if (q && sessionAnswers.value[q.id]) {
      map[i] = true;
    }
  }
  return map;
});

function jumpToQuestion(index: number) {
  currentIndex.value = index;
  questionStartTime.value = Date.now();
  showResult.value = false;
  result.value = {};
  applyAnswer(questions.value[index] || null);
}

useQuizShortcuts({
  enabled: () => mode.value === "practice" && !!currentQuestion.value,
  onSelect: (label) => {
    if (currentOptions.value.some((o) => o.label === label)) selectOption(label);
  },
  onEnter: () => {
    if (showResult.value) nextQuestion();
    else if (canSubmit.value) submitAnswer();
  },
  onPrev: () => {
    if (currentIndex.value > 0) jumpToQuestion(currentIndex.value - 1);
  },
  onNext: () => {
    if (currentIndex.value < questions.value.length - 1)
      jumpToQuestion(currentIndex.value + 1);
  },
});

</script>

<template>
  <!-- List Mode -->
  <div v-if="mode === 'list'" class="max-w-6xl mx-auto">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-6">
      分类刷题
    </h1>

    <div class="flex flex-wrap gap-2 sm:gap-3 mb-6">
      <button
        v-for="t in [1, 3, 4]"
        :key="t"
        @click="activeType = t"
        class="flex items-center gap-1.5 sm:gap-2 px-3 sm:px-5 py-2 sm:py-2.5 rounded-lg border text-sm font-medium transition-all"
        :class="
          activeType === t
            ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
            : 'border-gray-200 dark:border-gray-600 text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700/50'
        "
      >
        <el-icon :size="18"><component :is="typeIcons[t]" /></el-icon>
        {{ typeLabels[t] }}
        <span class="text-xs text-gray-400 dark:text-gray-500">
          ({{
            categories
              .filter((c) => c.question_type === t)
              .reduce((s, c) => s + c.count, 0)
          }})
        </span>
      </button>
    </div>

    <div v-if="loading" class="text-center py-12 text-gray-500">加载中...</div>
    <div
      v-else-if="filteredCategories.length === 0"
      class="text-center py-12 text-gray-500 dark:text-gray-400"
    >
      暂无分类数据
    </div>
    <div v-else class="space-y-3">
      <div
        v-for="cat in filteredCategories"
        :key="cat.main_cat"
        class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden transition-colors"
      >
        <div
          class="flex items-center justify-between px-5 py-3.5 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
          @click="toggleMainCat(cat.main_cat)"
        >
          <div class="flex items-center gap-3">
            <el-icon
              class="transition-transform"
              :class="{ 'rotate-90': expandedMainCats.has(cat.main_cat) }"
              :size="16"
              ><ArrowRight
            /></el-icon>
            <span class="font-semibold text-gray-800 dark:text-gray-200">{{
              cat.main_cat
            }}</span>
          </div>
          <div class="flex items-center gap-3">
            <span
              v-if="userStore.isLoggedIn && mainCatTotal(cat.subs) > 0"
              class="text-xs text-gray-500 dark:text-gray-400"
            >
              {{ mainCatDone(cat.subs) }}/{{ mainCatTotal(cat.subs) }}
            </span>
            <span class="text-sm text-gray-500 dark:text-gray-400"
              >{{ mainCatTotal(cat.subs) }} 题</span
            >
          </div>
        </div>
        <div
          v-if="expandedMainCats.has(cat.main_cat)"
          class="border-t border-gray-100 dark:border-gray-700"
        >
          <div
            v-for="sub in cat.subs"
            :key="sub.sub_cat"
            class="flex items-center justify-between px-5 py-3 hover:bg-blue-50/50 dark:hover:bg-blue-900/10 cursor-pointer transition-colors border-b border-gray-50 dark:border-gray-700/50 last:border-b-0"
            @click="openStartDialog(sub.sub_cat, sub.count)"
          >
            <div class="flex-1 min-w-0">
              <div class="text-sm text-gray-700 dark:text-gray-300 truncate">
                {{ sub.sub_cat }}
              </div>
              <div
                v-if="userStore.isLoggedIn && progressMap[sub.sub_cat]"
                class="mt-1.5 flex items-center gap-2"
              >
                <div
                  class="flex-1 max-w-[160px] h-1.5 bg-gray-100 dark:bg-gray-700 rounded-full overflow-hidden"
                >
                  <div
                    class="h-full bg-blue-500 rounded-full transition-all"
                    :style="{
                      width:
                        Math.round(
                          (progressMap[sub.sub_cat].done / sub.count) * 100,
                        ) + '%',
                    }"
                  ></div>
                </div>
                <span
                  class="text-xs text-gray-400 dark:text-gray-500 whitespace-nowrap"
                >
                  {{ progressMap[sub.sub_cat].done }}/{{ sub.count }}
                  <span
                    v-if="progressMap[sub.sub_cat].done > 0"
                    class="text-green-500"
                    >✓{{ progressMap[sub.sub_cat].correct }}</span
                  >
                </span>
              </div>
            </div>
            <div class="flex items-center gap-2 ml-3">
              <span
                class="text-xs text-gray-400 dark:text-gray-500 bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded"
                >{{ sub.count }}</span
              >
              <el-icon class="text-gray-400" :size="14"><ArrowRight /></el-icon>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Practice Mode -->
  <div v-else class="max-w-6xl mx-auto px-4">
    <div class="mb-4 flex items-center gap-3">
      <el-button @click="backToList" size="small"
        ><el-icon class="mr-1"><ArrowLeft /></el-icon> 返回</el-button
      >
      <span class="text-sm text-gray-500 dark:text-gray-400 truncate">{{
        currentKP
      }}</span>
      <span
        class="text-sm text-gray-500 dark:text-gray-400 ml-auto whitespace-nowrap"
        >{{ currentIndex + 1 }}/{{ questions.length }}</span
      >
    </div>
    <el-progress
      :percentage="Math.round((currentIndex / questions.length) * 100)"
      class="mb-4"
    />

    <div class="flex gap-6">
      <!-- Question Area -->
      <div class="flex-1 min-w-0">
        <div
          v-if="currentQuestion"
          class="bg-white dark:bg-gray-800 p-4 sm:p-6 rounded-lg shadow transition-colors"
        >
          <div
            v-if="!stemCoversTitle(currentQuestion.title, currentQuestion.content)"
            class="text-lg font-medium mb-4 whitespace-pre-wrap text-gray-900 dark:text-gray-100"
          >
            {{ currentQuestion.title }}
          </div>
          <div
            class="text-gray-600 dark:text-gray-400 mb-6 md-content leading-relaxed"
            v-html="renderedStem"
          ></div>

          <div class="mb-2 flex flex-wrap items-center gap-2">
            <el-tag v-if="totalBlanks > 1" type="warning" effect="plain" size="small" round>
              多空题 · 第 {{ currentBlank + 1 }}/{{ totalBlanks }} 空
            </el-tag>
          </div>

          <div
            v-if="totalBlanks > 1"
            class="mb-3 flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700 dark:border-blue-800/60 dark:bg-blue-950/40 dark:text-blue-300"
          >
            <span class="inline-flex h-6 min-w-6 items-center justify-center rounded-full bg-blue-600 px-2 text-xs text-white">{{ activeBlankNo }}</span>
            <span>当前选项对应：第 {{ currentBlank + 1 }} 空 / 共 {{ totalBlanks }} 空</span>
          </div>

          <div v-if="totalBlanks > 1" class="mb-4 flex gap-2 flex-wrap">
            <el-button
              v-for="(_, i) in blankGroups"
              :key="i"
              :type="
                currentBlank === i ? 'primary' : answers[i] ? 'success' : 'info'
              "
              size="small"
              @click="currentBlank = i"
            >
              第{{ i + 1 }}空 {{ answers[i] ? `(${answers[i]})` : "" }}
            </el-button>
          </div>

          <div v-if="currentOptions.length > 0" class="mb-6 space-y-3">
<div
              v-for="opt in currentOptions"
              :key="opt.id"
              class="p-3 border border-gray-200 dark:border-gray-600 rounded-lg cursor-pointer transition text-gray-800 dark:text-gray-200"
              :class="{
                'border-blue-500 bg-blue-50 dark:bg-blue-900/30':
                  !showResult && selectedAnswer === opt.label,
                'border-green-500 bg-green-50 dark:bg-green-900/30':
                  showResult && isCurrentBlankCorrect(opt.label),
                'border-red-500 bg-red-50 dark:bg-red-900/30':
                  showResult &&
                  selectedAnswer === opt.label &&
                  !isCurrentBlankCorrect(opt.label),
              }"
              @click="selectOption(opt.label)"
            >
              <span class="font-medium mr-2">{{ opt.label }}.</span
              ><span v-html="renderMdInline(opt.content)"></span>
            </div>
          </div>

          <div
            v-if="totalBlanks > 1 && showResult"
            class="mb-4 text-green-600 dark:text-green-400 font-medium"
          >
            正确答案：{{ correctAnswer }}
          </div>

          <div v-if="currentOptions.length === 0">
            <div v-if="!showResult">
              <div class="mb-4">
                <el-input
                  v-model="currentInput"
                  type="textarea"
                  :rows="5"
                  placeholder="请输入你的答案"
                />
              </div>
              <el-button
                type="primary"
                @click="submitAnswer"
                :disabled="!canSubmit"
                >提交</el-button
              >
            </div>
            <div v-else>
              <!-- 无选项＝案例/论文，系统判不了对错，不显示「回答错误」 -->
              <el-alert
                title="已提交，请对照下方参考答案自评"
                type="info"
                class="mb-4"
              />
              <div
                v-if="result.explanation"
                class="text-gray-600 dark:text-gray-400 mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 rounded"
              >
                <div class="font-medium">解析：</div>
                <div
                  class="md-content"
                  v-html="renderMd(result.explanation)"
                ></div>
              </div>
              <div
                v-if="result.analysis"
                class="text-gray-600 dark:text-gray-400 mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 rounded"
              >
                <div class="font-medium">参考答案 / 分析：</div>
                <div
                  class="md-content"
                  v-html="renderMd(result.analysis)"
                ></div>
              </div>
              <AIExplain
                v-if="currentQuestion"
                :question="currentQuestion"
                :answer="result.answer"
                class="mb-4"
              />
              <AIGradeCase
                v-if="currentQuestion"
                :question="currentQuestion"
                :user-answer="sessionAnswers[currentQuestion.id] || currentInput"
                class="mb-4"
              />
              <QuestionNoteEditor
                v-if="currentQuestion"
                :question-id="currentQuestion.id"
                class="mb-4"
              />
              <el-button type="primary" @click="nextQuestion">{{
                currentIndex < questions.length - 1 ? "下一题" : "完成"
              }}</el-button>
            </div>
          </div>
          <template v-else>
            <div v-if="!showResult">
              <div v-if="totalBlanks > 1" class="flex gap-2 mb-2">
                <el-button
                  :disabled="currentBlank === 0"
                  @click="currentBlank--"
                  >上一空</el-button
                >
                <el-button
                  v-if="currentBlank < totalBlanks - 1"
                  @click="currentBlank++"
                  >下一空</el-button
                >
              </div>
              <el-button
                type="primary"
                @click="submitAnswer"
                :disabled="!canSubmit"
                >提交</el-button
              >
              <ShortcutHint />
            </div>
            <div v-else>
              <el-alert
                :title="result.is_correct ? '回答正确！' : '回答错误'"
                :type="result.is_correct ? 'success' : 'error'"
                class="mb-4"
              />
              <div
                v-if="result.explanation"
                class="text-gray-600 dark:text-gray-400 mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 rounded"
              >
                <div class="font-medium">解析：</div>
                <div
                  class="md-content"
                  v-html="renderMd(result.explanation)"
                ></div>
              </div>
              <div
                v-if="result.analysis"
                class="text-gray-600 dark:text-gray-400 mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 rounded"
              >
                <div class="font-medium">分析：</div>
                <div
                  class="md-content"
                  v-html="renderMd(result.analysis)"
                ></div>
              </div>
              <AIExplain
                v-if="currentQuestion"
                :question="currentQuestion"
                :answer="result.answer"
                class="mb-4"
              />
              <QuestionNoteEditor
                v-if="currentQuestion"
                :question-id="currentQuestion.id"
                class="mb-4"
              />
              <el-button type="primary" @click="nextQuestion">{{
                currentIndex < questions.length - 1 ? "下一题" : "完成"
              }}</el-button>
            </div>
          </template>
        </div>
        <div v-else class="text-center py-12 text-gray-500 dark:text-gray-400">
          该知识点暂无题目
          <div class="mt-4">
            <el-button @click="backToList">返回</el-button>
          </div>
        </div>
      </div>

      <!-- Answer Card (Right Side) -->
      <div class="w-72 flex-shrink-0 hidden lg:block">
        <AnswerCard
          :total="questions.length"
          :current-index="currentIndex"
          :answered-map="answeredMap"
          class="sticky top-20"
          @jump="jumpToQuestion"
        />
      </div>
    </div>

    <!-- Mobile Answer Card (Bottom) -->
    <div class="lg:hidden mt-4">
      <AnswerCard
        :total="questions.length"
        :current-index="currentIndex"
        :answered-map="answeredMap"
        @jump="jumpToQuestion"
      />
    </div>
  </div>

  <!-- Start Dialog -->
  <el-dialog
    v-model="showStartDialog"
    title="开始刷题"
    width="400px"
    :close-on-click-modal="false"
  >
    <div class="py-2">
      <div class="text-gray-700 dark:text-gray-300 mb-2">{{ startKP }}</div>
      <div
        v-if="startDone > 0"
        class="text-sm text-gray-500 dark:text-gray-400 mb-2"
      >
        已完成 <span class="font-medium text-blue-500">{{ startDone }}</span> /
        {{ startTotal }} 题
      </div>
      <div v-else class="text-sm text-gray-500 dark:text-gray-400 mb-2">
        共 {{ startTotal }} 题，尚未开始
      </div>
      <div
        v-if="startResumable"
        class="text-xs text-gray-400 dark:text-gray-500 mb-4"
      >
        有上次未完成的进度，「继续刷题」会恢复上次的题目与答题卡；
        「从头开始」会清空这些答题记录，重新抽题。
      </div>
    </div>
    <template #footer>
      <div class="flex gap-3 justify-end">
        <el-button @click="showStartDialog = false">取消</el-button>
        <el-button
          v-if="startResumable"
          type="primary"
          :loading="starting"
          @click="startPractice(false)"
          >继续刷题</el-button
        >
        <el-button
          :type="startResumable ? 'default' : 'primary'"
          :loading="starting"
          @click="startPractice(true)"
          >从头开始</el-button
        >
      </div>
    </template>
  </el-dialog>
</template>
