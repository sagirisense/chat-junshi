"""微信军师：手动采集、核对、分析和复制。"""
import ctypes
import multiprocessing as mp
from pathlib import Path
import queue
import threading
import tkinter as tk
from tkinter import filedialog, messagebox, ttk

from codex_bridge import analyze
from collector import run
from sessions import Session

ROOT = Path(__file__).resolve().parent


class Window:
    def __init__(self, root):
        self.root = root
        self.sessions = {}
        self.current = None
        self.child = None
        self.enabled = mp.Event()
        self.incoming = mp.Queue()
        self.results = queue.Queue()
        self.busy = False
        self.blocked = False
        self.capture_epoch = 0
        root.title('微信军师 · Codex')
        root.geometry('600x850')
        root.minsize(480, 700)
        root.protocol('WM_DELETE_WINDOW', self.close)
        toolbar = ttk.Frame(root, padding=10)
        toolbar.pack(fill='x')
        self.capture_button = ttk.Button(toolbar, text='开启采集', command=self.toggle)
        self.capture_button.pack(side='left')
        self.topmost = tk.BooleanVar(value=True)
        ttk.Checkbutton(toolbar, text='置顶', variable=self.topmost,
                        command=lambda: root.attributes('-topmost', self.topmost.get())).pack(side='right')
        root.attributes('-topmost', True)
        self.title = tk.StringVar(value='先打开微信单聊，再开启采集')
        ttk.Label(root, textvariable=self.title, padding=10).pack(fill='x')
        self.status = tk.StringVar(value='已暂停；仅在点击分析时向 Codex 提交文字')
        ttk.Label(root, textvariable=self.status, wraplength=550, padding=10).pack(fill='x')
        self.confirmed = tk.BooleanVar()
        ttk.Checkbutton(root, text='我已核对下方消息：用户=我；对象A=对方；当前为单聊',
                        variable=self.confirmed, command=self.confirm).pack(anchor='w', padx=10)
        profile = ttk.Frame(root, padding=10)
        profile.pack(fill='x')
        ttk.Label(profile, text='关系背景').grid(row=0, column=0)
        self.background = ttk.Entry(profile)
        self.background.grid(row=0, column=1, sticky='ew', padx=6)
        ttk.Label(profile, text='眼下目标').grid(row=1, column=0)
        self.goal = ttk.Entry(profile)
        self.goal.grid(row=1, column=1, sticky='ew', padx=6, pady=5)
        profile.columnconfigure(1, weight=1)
        ttk.Label(profile, text='Codex 模型').grid(row=2, column=0)
        self.model = ttk.Entry(profile)
        self.model.grid(row=2, column=1, sticky='ew', padx=6)
        ttk.Label(profile, text='留空使用 CLI 返回的可用默认模型').grid(row=3, column=1, sticky='w')
        self.table = ttk.Treeview(root, columns=('sender', 'text'), show='headings', height=8)
        self.table.heading('sender', text='说话人')
        self.table.heading('text', text='采集原文（双击纠错）')
        self.table.column('sender', width=75, stretch=False)
        self.table.column('text', width=440)
        self.table.pack(fill='x', padx=10)
        self.table.bind('<Double-1>', self.correct)
        actions = ttk.Frame(root, padding=10)
        actions.pack(fill='x')
        self.analyze_button = ttk.Button(actions, text='军师分析', command=self.start_analysis)
        self.analyze_button.pack(side='left')
        ttk.Button(actions, text='复制 Skill 输入', command=self.copy_packet).pack(side='left', padx=6)
        ttk.Button(actions, text='导出 JSON / Markdown', command=self.export).pack(side='left')
        ttk.Button(actions, text='清空', command=self.clear).pack(side='right')
        self.output = tk.Text(root, height=10, wrap='word', state='disabled')
        self.output.pack(fill='both', expand=True, padx=10)
        card_area = ttk.Frame(root)
        card_area.pack(fill='both', expand=True, padx=10, pady=5)
        canvas = tk.Canvas(card_area, height=280, highlightthickness=0)
        scrollbar = ttk.Scrollbar(card_area, orient='vertical', command=canvas.yview)
        canvas.configure(yscrollcommand=scrollbar.set)
        scrollbar.pack(side='right', fill='y')
        canvas.pack(side='left', fill='both', expand=True)
        card_content = ttk.Frame(canvas)
        card_window = canvas.create_window((0, 0), window=card_content, anchor='nw')
        card_content.bind('<Configure>', lambda e: canvas.configure(scrollregion=canvas.bbox('all')))
        def resize_cards(event):
            canvas.itemconfigure(card_window, width=event.width)
            for _, label, _ in self.cards:
                label.configure(wraplength=max(250, event.width - 24))
        canvas.bind('<Configure>', resize_cards)
        self.cards = []
        for number in range(3):
            frame = ttk.LabelFrame(card_content, text=f'候选 {number + 1}', padding=6)
            frame.pack(fill='x', pady=3)
            label = ttk.Label(frame, wraplength=540)
            label.pack(fill='x')
            button = ttk.Button(frame, text='复制回复', state='disabled')
            button.pack(anchor='e')
            self.cards.append((frame, label, button))
        root.after(100, self.tick)

    def session(self, title):
        if title not in self.sessions:
            self.sessions[title] = Session(title, ROOT / 'data')
        return self.sessions[title]

    def profile_save(self):
        if self.current:
            if (self.current.background != self.background.get() or
                    self.current.goal != self.goal.get()):
                self.current.revision += 1
            self.current.background = self.background.get()
            self.current.goal = self.goal.get()
            self.current.save()

    def select(self, title):
        if self.current and self.current.title == title:
            return
        self.profile_save()
        self.current = self.session(title)
        # A title alone is not a stable contact ID. Reconfirm on every switch.
        self.current.confirmed = False
        self.confirmed.set(False)
        self.background.delete(0, 'end')
        self.background.insert(0, self.current.background)
        self.goal.delete(0, 'end')
        self.goal.insert(0, self.current.goal)
        self.title.set(f'当前会话：{title}')
        self.refresh()
        self.invalidate()

    def refresh(self):
        self.table.delete(*self.table.get_children())
        if self.current:
            for index, msg in enumerate(self.current.messages):
                self.table.insert('', 'end', iid=str(index), values=(
                    '用户' if msg['sender'] == 'me' else '对象A', msg['text']))
            children = self.table.get_children()
            if children:
                self.table.see(children[-1])

    def confirm(self):
        if self.current and not self.blocked:
            self.current.confirmed = self.confirmed.get()
            self.current.revision += 1
            self.invalidate()
        else:
            self.confirmed.set(False)

    def invalidate(self):
        self.output.configure(state='normal')
        self.output.delete('1.0', 'end')
        self.output.configure(state='disabled')
        for frame, label, button in self.cards:
            label.configure(text='')
            button.configure(state='disabled')

    def toggle(self):
        if self.enabled.is_set():
            self.enabled.clear()
            self.capture_epoch += 1
            self.capture_button.configure(text='开启采集')
            self.status.set('已暂停采集')
            return
        if self.child is None or not self.child.is_alive():
            self.child = mp.Process(target=run, args=(self.incoming, self.enabled), daemon=True)
            self.child.start()
        self.capture_epoch += 1
        self.enabled.set()
        self.capture_button.configure(text='暂停采集')
        self.status.set('采集中：请核对识别原文和当前会话')

    def correct(self, event=None):
        if not self.current or not self.table.selection():
            return
        index = int(self.table.selection()[0])
        msg = self.current.messages[index]
        dialog = tk.Toplevel(self.root)
        dialog.title('纠正消息')
        dialog.transient(self.root)
        dialog.grab_set()
        sender = tk.StringVar(value=msg['sender'])
        ttk.Radiobutton(dialog, text='用户（我）', variable=sender, value='me').pack(anchor='w')
        ttk.Radiobutton(dialog, text='对象A（对方）', variable=sender, value='peer').pack(anchor='w')
        text = tk.Text(dialog, width=55, height=8)
        text.pack(padx=10, pady=10)
        text.insert('1.0', msg['text'])
        selected = self.current

        def save():
            try:
                selected.correct(index, sender.get(), text.get('1.0', 'end'))
            except ValueError as exc:
                messagebox.showerror('无法保存', str(exc), parent=dialog)
                return
            dialog.destroy()
            self.refresh()
            self.invalidate()
        ttk.Button(dialog, text='保存', command=save).pack(pady=5)

    def packet(self):
        if not self.current or self.blocked:
            raise ValueError('请先采集一个标题明确的单聊会话')
        self.profile_save()
        return self.current.packet()

    def copy(self, text):
        self.root.clipboard_clear()
        self.root.clipboard_append(text)
        self.status.set('已复制，可自行粘贴；不会自动发送')

    def copy_packet(self):
        try:
            self.copy(self.packet())
        except ValueError as exc:
            messagebox.showinfo('尚不能分析', str(exc))

    def export(self):
        try:
            packet = self.packet()
        except ValueError as exc:
            messagebox.showinfo('尚不能导出', str(exc))
            return
        path = filedialog.asksaveasfilename(defaultextension='.md',
                                          filetypes=[('Skill 输入', '*.md'), ('JSON 数据', '*.json')])
        if path:
            import json
            content = json.dumps(self.current.payload(), ensure_ascii=False, indent=2) if path.endswith('.json') else packet
            Path(path).write_text(content, encoding='utf-8')
            self.status.set('已导出当前会话')

    def clear(self):
        if self.current and messagebox.askyesno('清空会话', '删除当前会话的本地文字记录和背景？'):
            # Restart OCR state; otherwise deleted messages remain in its dedup buffer.
            if self.child:
                self.enabled.clear()
                self.child.terminate()
                self.child.join(timeout=3)
                self.child = None
                self.capture_button.configure(text='开启采集')
            self.capture_epoch += 1
            while True:
                try:
                    self.incoming.get_nowait()
                except queue.Empty:
                    break
            self.current.messages.clear()
            self.current.background = self.current.goal = ''
            self.current.confirmed = False
            self.current.revision += 1
            self.current.save()
            self.confirmed.set(False)
            self.background.delete(0, 'end')
            self.goal.delete(0, 'end')
            self.refresh()
            self.invalidate()
            self.status.set('已清空；采集已暂停')

    def start_analysis(self):
        if self.busy:
            return
        try:
            packet = self.packet()
        except ValueError as exc:
            messagebox.showinfo('尚不能分析', str(exc))
            return
        title, revision = self.current.title, self.current.revision
        epoch = self.capture_epoch
        selected_model = self.model.get().strip() or None
        self.busy = True
        self.analyze_button.configure(state='disabled')
        self.status.set('Codex 正在调用 goutoujunshi，通常需要几十秒…')

        def work():
            try:
                self.results.put((title, revision, epoch, analyze(packet, model=selected_model), None))
            except Exception as exc:
                self.results.put((title, revision, epoch, None, str(exc)))
        threading.Thread(target=work, daemon=True).start()

    def show_result(self, result):
        text = result['summary'] + '\n推荐理由：' + result['recommendation_reason']
        for label, key in [('事实', 'facts'), ('推测', 'hypotheses'), ('未知', 'unknowns')]:
            text += '\n\n' + label + '\n' + '\n'.join(result[key])
        self.output.configure(state='normal')
        self.output.delete('1.0', 'end')
        self.output.insert('1.0', text)
        self.output.configure(state='disabled')
        for index, candidate in enumerate(result['candidates']):
            frame, label, button = self.cards[index]
            frame.configure(text=f'候选 {index + 1}' + (' · 推荐' if index == result['recommended_index'] else ''))
            label.configure(text=candidate['text'] + '\n优点：' + '；'.join(candidate['pros'])
                            + '\n代价：' + '；'.join(candidate['cons'])
                            + '\n适用：' + candidate['suitable_when']
                            + '\n后续：' + candidate['next_step'])
            button.configure(state='normal' if candidate['text'] else 'disabled',
                             command=lambda text=candidate['text']: self.copy(text))

    def tick(self):
        try:
            self.drain()
        except Exception as exc:
            self.status.set(f'处理失败：{type(exc).__name__}，请检查本地文件或采集依赖')
        self.root.after(100, self.tick)

    def drain(self):
        for _ in range(100):
            try:
                event = self.incoming.get_nowait()
            except queue.Empty:
                break
            kind = event[0]
            if kind == 'dead':
                self.enabled.clear()
                self.capture_epoch += 1
                self.blocked = True
                self.confirmed.set(False)
                if self.current:
                    self.current.confirmed = False
                self.capture_button.configure(text='开启采集')
                self.status.set(event[1])
                self.invalidate()
            elif not self.enabled.is_set():
                continue
            elif kind == 'chat':
                self.blocked = False
                self.select(event[1])
            elif kind == 'lines' and not self.blocked:
                self.select(event[1])
                self.current.append(event[2])
                self.refresh()
                self.invalidate()
                self.status.set('已收到消息；核对后可点击军师分析')
            elif kind in ('status', 'blocked'):
                self.status.set(event[1])
                if kind == 'blocked':
                    self.blocked = True
                    self.capture_epoch += 1
                    self.confirmed.set(False)
                    if self.current:
                        self.current.confirmed = False
                    self.invalidate()
        try:
            title, revision, epoch, result, error = self.results.get_nowait()
        except queue.Empty:
            return
        self.busy = False
        self.analyze_button.configure(state='normal')
        self.profile_save()
        if (not self.current or self.current.title != title or self.current.revision != revision
                or self.capture_epoch != epoch or self.blocked):
            self.status.set('会话或消息已变化，旧分析已丢弃；请重新分析')
            return
        self.invalidate()
        if error:
            self.status.set('分析失败：' + error)
        else:
            self.show_result(result)
            self.status.set('分析完成；请核对后自行复制发送')

    def close(self):
        self.profile_save()
        if self.child:
            self.child.terminate()
            self.child.join(timeout=3)
        self.root.destroy()


if __name__ == '__main__':
    mp.freeze_support()
    ctypes.windll.user32.SetProcessDPIAware()
    root = tk.Tk()
    Window(root)
    root.mainloop()
