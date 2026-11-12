import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Page, PageHeader, Card, Button, TextField, TextArea, EmptyState, Badge, Spinner } from "../ds";
import { Plus, Trash2, ArrowRight } from "lucide-react";
import { api } from "../api";
import type { Brand } from "../types";

export default function BrandsPage() {
  const [brands, setBrands] = useState<Brand[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({
    name: "",
    website: "",
    description: "",
    competitors: "",
    keywords: "",
  });

  const load = () => {
    api.listBrands().then(setBrands).catch(() => {}).finally(() => setLoading(false));
  };

  useEffect(() => load(), []);

  const submit = () => {
    api
      .createBrand({
        name: form.name,
        website: form.website,
        description: form.description,
        competitors: form.competitors.split(",").map((s) => s.trim()).filter(Boolean),
        keywords: form.keywords.split(",").map((s) => s.trim()).filter(Boolean),
      })
      .then(() => {
        setForm({ name: "", website: "", description: "", competitors: "", keywords: "" });
        setShowForm(false);
        load();
      });
  };

  const remove = (id: string) => {
    if (!confirm("确认删除？")) return;
    api.deleteBrand(id).then(load);
  };

  return (
    <Page>
      <PageHeader
        title="品牌管理"
        description="添加你要监控的品牌和竞品"
        actions={
          <Button onClick={() => setShowForm(!showForm)}>
            <Plus className="h-4 w-4" />
            添加品牌
          </Button>
        }
      />

      {showForm && (
        <Card className="mb-6">
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <TextField
                label="品牌名称"
                placeholder="如：Notion"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
              <TextField
                label="网站"
                placeholder="https://notion.so"
                value={form.website}
                onChange={(e) => setForm({ ...form, website: e.target.value })}
              />
            </div>
            <TextArea
              label="品牌描述"
              placeholder="一句话描述你的产品或服务"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
            <div className="grid grid-cols-2 gap-4">
              <TextField
                label="竞品（逗号分隔）"
                placeholder="Obsidian, Evernote, Roam"
                value={form.competitors}
                onChange={(e) => setForm({ ...form, competitors: e.target.value })}
              />
              <TextField
                label="关键词（逗号分隔）"
                placeholder="笔记, 知识管理, 协作"
                value={form.keywords}
                onChange={(e) => setForm({ ...form, keywords: e.target.value })}
              />
            </div>
            <div className="flex gap-2">
              <Button onClick={submit} disabled={!form.name}>
                保存
              </Button>
              <Button variant="ghost" onClick={() => setShowForm(false)}>
                取消
              </Button>
            </div>
          </div>
        </Card>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <Spinner />
        </div>
      ) : brands.length === 0 ? (
        <Card>
          <EmptyState
            title="还没有添加品牌"
            description="添加你的品牌，开始检测在 AI 模型中的推荐表现"
            action={<Button onClick={() => setShowForm(true)}>添加第一个品牌</Button>}
          />
        </Card>
      ) : (
        <div className="space-y-3">
          {brands.map((brand) => (
            <Card key={brand.id} className="flex items-center justify-between">
              <div className="flex-1">
                <div className="flex items-center gap-3">
                  <h3 className="text-base font-medium text-ink">{brand.name}</h3>
                  {brand.website && (
                    <a
                      href={brand.website}
                      target="_blank"
                      rel="noopener"
                      className="text-xs text-accent hover:underline"
                    >
                      {brand.website}
                    </a>
                  )}
                </div>
                {brand.description && (
                  <p className="mt-1 text-sm text-ink-muted">{brand.description}</p>
                )}
                <div className="mt-2 flex gap-2 flex-wrap">
                  {brand.competitors?.map((c) => (
                    <Badge key={c} tone="neutral">
                      {c}
                    </Badge>
                  ))}
                  {brand.keywords?.map((k) => (
                    <Badge key={k} tone="accent">
                      {k}
                    </Badge>
                  ))}
                </div>
              </div>
              <div className="flex items-center gap-2 ml-4">
                <Link to={`/brands/${brand.id}`}>
                  <Button variant="secondary" size="sm">
                    查看报告
                    <ArrowRight className="h-4 w-4" />
                  </Button>
                </Link>
                <Button variant="ghost" size="sm" onClick={() => remove(brand.id)}>
                  <Trash2 className="h-4 w-4 text-negative" />
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}
    </Page>
  );
}
