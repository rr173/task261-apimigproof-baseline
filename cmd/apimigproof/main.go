// apimigproof 跨版本 API 语义弃用迁移证明服务入口。
//
// 用法：
//
//	apimigproof --addr :8080 --db ./apimigproof.db    启动 HTTP 服务
//	apimigproof --smoke-test                           自检：真实建库→比较闭环→
//	                                                   关闭重开验证恢复→退出 0
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"task261-apimigproof/internal/httpapi"
	"task261-apimigproof/internal/model"
	"task261-apimigproof/internal/service"
	"task261-apimigproof/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "./apimigproof.db", "sqlite database path")
	smoke := flag.Bool("smoke-test", false, "run closed-loop self test and exit 0 on success")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "smoke-test FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("smoke-test OK")
		return
	}

	st, err := store.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()
	svc := service.New(st)
	srv := &http.Server{Addr: *addr, Handler: httpapi.New(svc).Handler()}
	log.Printf("apimigproof listening on %s (db=%s)", *addr, *dbPath)
	log.Fatal(srv.ListenAndServe())
}

// smokeCheck 断言辅助：条件不满足时返回错误并中止自检。
func smokeCheck(cond bool, format string, args ...interface{}) error {
	if !cond {
		return fmt.Errorf(format, args...)
	}
	return nil
}

// runSmokeTest 执行真实闭环自检：
//
//  1. 真实建库，执行完整端到端场景（契约 v1/v2、转换规则、样本导入、
//     歧义比较、兼容规则补充、兼容比较、证明发布）；
//  2. 关闭并重新打开同一数据库，验证契约、字段、规则、样本状态、
//     比较结果与已发布证明全部恢复（重启恢复语义）；
//  3. 全部断言通过返回 nil（退出码 0）。
func runSmokeTest(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		dir = "."
	}
	path := filepath.Join(dir, "smoke-apimigproof.db")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	// ---- 第一轮：写入 ----
	st, err := store.OpenStore(path)
	if err != nil {
		return err
	}
	svc := service.New(st)
	v1ID, v2ID, err := svc.SelfCheck()
	if err != nil {
		st.Close()
		return fmt.Errorf("selfcheck: %w", err)
	}

	// 记录第一轮产生的关键 ID 与状态。
	v1, err := svc.Store.GetContract(v1ID)
	if err != nil {
		st.Close()
		return fmt.Errorf("get v1: %w", err)
	}
	v2, err := svc.Store.GetContract(v2ID)
	if err != nil {
		st.Close()
		return fmt.Errorf("get v2: %w", err)
	}
	if err := smokeCheck(v1.Status == model.ContractSealed && v2.Status == model.ContractSealed,
		"contracts after selfcheck: v1=%s v2=%s", v1.Status, v2.Status); err != nil {
		st.Close()
		return err
	}
	samples, err := svc.Store.ListSamples()
	if err != nil {
		st.Close()
		return err
	}
	if err := smokeCheck(len(samples) == 2, "samples = %d, want 2", len(samples)); err != nil {
		st.Close()
		return err
	}
	comps, err := svc.Store.ListComparisons()
	if err != nil {
		st.Close()
		return err
	}
	if err := smokeCheck(len(comps) == 2, "comparisons = %d, want 2", len(comps)); err != nil {
		st.Close()
		return err
	}
	// 列表按 ID 倒序：comps[0] 为最新（第二次=compatible），comps[1] 为最早（第一次=ambiguous）。
	if err := smokeCheck(comps[0].Status == model.ComparisonCompatible && comps[1].Status == model.ComparisonAmbiguous,
		"comparison statuses: %s, %s", comps[0].Status, comps[1].Status); err != nil {
		st.Close()
		return err
	}
	proofs, err := svc.Store.ListProofs()
	if err != nil {
		st.Close()
		return err
	}
	if err := smokeCheck(len(proofs) == 1 && proofs[0].Status == model.ProofPublished,
		"proofs = %d status=%s, want 1/published", len(proofs), proofs[0].Status); err != nil {
		st.Close()
		return err
	}
	publishedProof := proofs[0]

	// ---- 关闭重开：验证持久化恢复 ----
	if err := st.Close(); err != nil {
		return err
	}
	st2, err := store.OpenStore(path)
	if err != nil {
		return fmt.Errorf("reopen store: %w", err)
	}
	defer st2.Close()
	svc2 := service.New(st2)

	v1b, err := svc2.Store.GetContract(v1.ID)
	if err != nil {
		return fmt.Errorf("recover v1: %w", err)
	}
	if err := smokeCheck(v1b.Name == v1.Name && v1b.Status == v1.Status,
		"v1 recovered: name=%q status=%q", v1b.Name, v1b.Status); err != nil {
		return err
	}
	fieldsV2, err := svc2.Store.ListFields(v2.ID)
	if err != nil {
		return fmt.Errorf("recover v2 fields: %w", err)
	}
	if err := smokeCheck(len(fieldsV2) == 3, "v2 fields recovered = %d, want 3", len(fieldsV2)); err != nil {
		return err
	}
	rulesV2, err := svc2.Store.ListRules(v2.ID)
	if err != nil {
		return fmt.Errorf("recover v2 rules: %w", err)
	}
	if err := smokeCheck(len(rulesV2) == 3, "v2 rules recovered = %d, want 3", len(rulesV2)); err != nil {
		return err
	}
	samplesB, err := svc2.Store.ListSamples()
	if err != nil {
		return fmt.Errorf("recover samples: %w", err)
	}
	if err := smokeCheck(len(samplesB) == 2, "samples recovered = %d, want 2", len(samplesB)); err != nil {
		return err
	}
	byStatus := map[string]int{}
	for _, sm := range samplesB {
		byStatus[sm.Status]++
	}
	if err := smokeCheck(byStatus[model.SampleMigratable] == 2,
		"sample statuses recovered = %v, want 2 migratable", byStatus); err != nil {
		return err
	}
	// 恢复最近一次比较（第二次，compatible/全可迁移）。
	comp2, err := svc2.Store.GetComparison(comps[0].ID)
	if err != nil {
		return fmt.Errorf("recover comparison: %w", err)
	}
	if err := smokeCheck(comp2.Status == model.ComparisonCompatible && comp2.Migratable == 2,
		"comparison recovered: status=%s migratable=%d", comp2.Status, comp2.Migratable); err != nil {
		return err
	}
	proofB, err := svc2.Store.GetProof(publishedProof.ID)
	if err != nil {
		return fmt.Errorf("recover proof: %w", err)
	}
	if err := smokeCheck(proofB.Status == model.ProofPublished &&
		proofB.EvidenceFingerprint == publishedProof.EvidenceFingerprint,
		"proof recovered: status=%s fingerprint=%s", proofB.Status, proofB.EvidenceFingerprint); err != nil {
		return err
	}
	// 封存契约恢复后仍不可修改。
	if _, err := svc2.Reg.AddField(model.FieldSemantics{
		ContractID: v1.ID, FieldID: "late_field", Status: model.FieldValid,
		ValueType: model.TypeString,
	}); err == nil {
		return fmt.Errorf("expected sealed contract to reject field add after restart")
	}

	_ = os.Remove(path)
	return nil
}
