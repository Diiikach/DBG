// Package enrichment — обогащение вариантов через MyVariant.info / ClinVar (модуль 7).
//
// MyVariant.info — публичный JSON API: GET https://myvariant.info/v1/variant/{hgvs}
// Возвращает поля dbnsfp/gnomad_genome/clinvar/cadd и пр. Мы извлекаем подмножество,
// которое умеет хранить таблица variant_annotation:
//   - gnomad_af, gnomad_af_popmax
//   - sift_score, sift_prediction
//   - polyphen_score, polyphen_prediction
//   - cadd_score
//   - revel_score
//   - clinvar_significance, clinvar_id
//   - gene_symbol (для upsert в gene)
//   - consequence/impact (по dbnsfp.ensembl)
package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MyVariantClient — HTTP-клиент MyVariant.info.
type MyVariantClient struct {
	http    *http.Client
	baseURL string // override для тестов
}

// NewMyVariantClient создаёт клиента с заданным HTTP-таймаутом
// (таймаут должен быть задан на http.Client.Timeout снаружи).
func NewMyVariantClient(httpc *http.Client) *MyVariantClient {
	return &MyVariantClient{
		http:    httpc,
		baseURL: "https://myvariant.info/v1/variant/",
	}
}

// VariantData — нормализованные данные из MyVariant.info, готовые к маппингу
// в строку variant_annotation. Все поля опциональны (могут быть nil).
type VariantData struct {
	GeneSymbol          *string
	Consequence         *string
	Impact              *string // HIGH/MODERATE/LOW/MODIFIER
	GnomadAF            *float64
	GnomadAFPopmax      *float64
	SIFTScore           *float64
	SIFTPrediction      *string
	PolyphenScore       *float64
	PolyphenPrediction  *string
	CADDScore           *float64
	RevelScore          *float64
	ClinvarSignificance *string
	ClinvarID           *string
}

// Fetch выполняет GET /v1/variant/{hgvs}?fields=... и парсит ответ.
// Если HGVS пустой — возвращает (nil, nil) (нет данных для обогащения).
// 404 трактуется как «нет данных», возвращается пустой объект и nil.
func (c *MyVariantClient) Fetch(ctx context.Context, hgvs string) (*VariantData, error) {
	hgvs = strings.TrimSpace(hgvs)
	if hgvs == "" {
		return nil, nil
	}
	u := c.baseURL + url.PathEscape(hgvs) +
		"?fields=dbnsfp,gnomad_genome,clinvar,cadd"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build req: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &VariantData{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("myvariant http %d: %s", resp.StatusCode, string(body))
	}

	var raw rawMyVariantResp
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return raw.toVariantData(), nil
}

// rawMyVariantResp — частичная структура ответа. Поля имеют гетерогенные типы
// (объект или массив объектов), поэтому используем json.RawMessage и парсим вручную.
type rawMyVariantResp struct {
	Dbnsfp       json.RawMessage `json:"dbnsfp"`
	GnomadGenome json.RawMessage `json:"gnomad_genome"`
	Clinvar      json.RawMessage `json:"clinvar"`
	Cadd         json.RawMessage `json:"cadd"`
}

func (r *rawMyVariantResp) toVariantData() *VariantData {
	out := &VariantData{}

	// --- dbnsfp: ген, SIFT, PolyPhen, REVEL ---
	if len(r.Dbnsfp) > 0 && string(r.Dbnsfp) != "null" {
		var d struct {
			Genename   json.RawMessage `json:"genename"`
			Sift       struct {
				Score      *float64 `json:"score"`
				Pred       *string  `json:"pred"`
			} `json:"sift"`
			Polyphen2 struct {
				HVAR struct {
					Score *float64 `json:"score"`
					Pred  *string  `json:"pred"`
				} `json:"hvar"`
			} `json:"polyphen2"`
			Revel struct {
				Score *float64 `json:"score"`
			} `json:"revel"`
			Ensembl struct {
				Consequence *string `json:"consequence"`
			} `json:"ensembl"`
		}
		if err := json.Unmarshal(r.Dbnsfp, &d); err == nil {
			if g := pickString(d.Genename); g != nil {
				out.GeneSymbol = g
			}
			if d.Sift.Score != nil {
				out.SIFTScore = d.Sift.Score
			}
			if d.Sift.Pred != nil {
				out.SIFTPrediction = d.Sift.Pred
			}
			if d.Polyphen2.HVAR.Score != nil {
				out.PolyphenScore = d.Polyphen2.HVAR.Score
			}
			if d.Polyphen2.HVAR.Pred != nil {
				out.PolyphenPrediction = d.Polyphen2.HVAR.Pred
			}
			if d.Revel.Score != nil {
				out.RevelScore = d.Revel.Score
			}
			if d.Ensembl.Consequence != nil {
				out.Consequence = d.Ensembl.Consequence
				out.Impact = impactFromConsequence(*d.Ensembl.Consequence)
			}
		}
	}

	// --- gnomad_genome ---
	if len(r.GnomadGenome) > 0 && string(r.GnomadGenome) != "null" {
		var g struct {
			AF struct {
				AF       *float64 `json:"af"`
				AFPopmax *float64 `json:"af_popmax"`
			} `json:"af"`
		}
		if err := json.Unmarshal(r.GnomadGenome, &g); err == nil {
			if g.AF.AF != nil {
				out.GnomadAF = g.AF.AF
			}
			if g.AF.AFPopmax != nil {
				out.GnomadAFPopmax = g.AF.AFPopmax
			}
		}
	}

	// --- clinvar ---
	if len(r.Clinvar) > 0 && string(r.Clinvar) != "null" {
		var c struct {
			RCV json.RawMessage `json:"rcv"`
			VariantID *int64    `json:"variant_id"`
		}
		if err := json.Unmarshal(r.Clinvar, &c); err == nil {
			if sig := pickClinvarSignificance(c.RCV); sig != nil {
				out.ClinvarSignificance = sig
			}
			if c.VariantID != nil {
				s := fmt.Sprintf("%d", *c.VariantID)
				out.ClinvarID = &s
			}
		}
	}

	// --- cadd ---
	if len(r.Cadd) > 0 && string(r.Cadd) != "null" {
		var c struct {
			Phred *float64 `json:"phred"`
		}
		if err := json.Unmarshal(r.Cadd, &c); err == nil && c.Phred != nil {
			out.CADDScore = c.Phred
		}
	}

	return out
}

// pickString извлекает строку из поля, которое может быть string или []string.
func pickString(raw json.RawMessage) *string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return &s
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, v := range arr {
			if v != "" {
				vv := v
				return &vv
			}
		}
	}
	return nil
}

// pickClinvarSignificance берёт первое непустое clinical_significance из RCV.
// RCV может быть объектом или массивом объектов.
func pickClinvarSignificance(raw json.RawMessage) *string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	type rcvItem struct {
		ClinicalSignificance string `json:"clinical_significance"`
	}
	var single rcvItem
	if err := json.Unmarshal(raw, &single); err == nil && single.ClinicalSignificance != "" {
		return &single.ClinicalSignificance
	}
	var arr []rcvItem
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, it := range arr {
			if it.ClinicalSignificance != "" {
				v := it.ClinicalSignificance
				return &v
			}
		}
	}
	return nil
}

// impactFromConsequence — грубая эвристика consequence→impact в стиле Ensembl VEP.
func impactFromConsequence(c string) *string {
	c = strings.ToLower(c)
	switch {
	case strings.Contains(c, "stop_gained"),
		strings.Contains(c, "frameshift"),
		strings.Contains(c, "splice_acceptor"),
		strings.Contains(c, "splice_donor"),
		strings.Contains(c, "start_lost"),
		strings.Contains(c, "stop_lost"):
		s := "HIGH"
		return &s
	case strings.Contains(c, "missense"),
		strings.Contains(c, "inframe"),
		strings.Contains(c, "protein_altering"):
		s := "MODERATE"
		return &s
	case strings.Contains(c, "synonymous"),
		strings.Contains(c, "splice_region"):
		s := "LOW"
		return &s
	default:
		s := "MODIFIER"
		return &s
	}
}
