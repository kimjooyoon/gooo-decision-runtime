package record

import old "github.com/kimjooyoon/gooo-decision-runtime/studies/contract-goals-20261010/record"

const Compiler = "aa39466774afe64e56ee87f419ea56fb50917abe"
const CorpusSHA = "7a28ccaef1d832c8e7259ed9371bb6933c4f53d7c066e556e58aaeabb39881a4"

type Candidate = old.Candidate
type Source = old.Source
type Training = old.Training
type Search = old.Search
type Group = old.Group
type Spec = old.Spec

type FrozenSource struct {
	Spec Spec
	Gooo string
}

var Hash = old.Hash
var Encode = old.Encode
var FeatureSHA = old.FeatureSHA
