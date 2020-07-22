// Copyright 2020 Liquidata, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package doltdb_test

import (
	"context"
	"github.com/liquidata-inc/dolt/go/cmd/dolt/commands"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liquidata-inc/dolt/go/cmd/dolt/cli"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/doltdb"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/dtestutils"
)

func TestForeignKeys(t *testing.T) {
	for _, test := range foreignKeyTests {
		t.Run(test.name, func(t *testing.T) {
			testForeignKeys(t, test)
		})
	}
}

type foreignKeyTest struct {
	name  string
	setup []testCommand
	fks   []doltdb.ForeignKey
}

type testCommand struct {
	cmd  cli.Command
	args []string
}

var setupCommon = []testCommand{
	{commands.SqlCmd{}, []string{"-q", "create table parent (" +
		"id int comment 'tag:0'," +
		"v1 int comment 'tag:1'," +
		"v2 int comment 'tag:2'," +
		//"index v1_idx (v1)," +
		//"index v1_idx (v2)," +
		"primary key(id));"}},
	{commands.SqlCmd{}, []string{"-q", "alter table parent add index v1_idx (v1);"}},
	{commands.SqlCmd{}, []string{"-q", "alter table parent add index v2_idx (v2);"}},
	{commands.SqlCmd{}, []string{"-q", "create table child (" +
		"id int comment 'tag:10', " +
		"v1 int comment 'tag:11'," +
		"v2 int comment 'tag:12'," +
		"primary key(id));"}},
}

func testForeignKeys(t *testing.T, test foreignKeyTest) {
	ctx := context.Background()
	dEnv := dtestutils.CreateTestEnv()

	for _, c := range setupCommon {
		exitCode := c.cmd.Exec(ctx, c.cmd.Name(), c.args, dEnv)
		require.Equal(t, 0, exitCode)
	}
	for _, c := range test.setup {
		exitCode := c.cmd.Exec(ctx, c.cmd.Name(), c.args, dEnv)
		require.Equal(t, 0, exitCode)
	}

	root, err := dEnv.WorkingRoot(ctx)
	require.NoError(t, err)
	fkc, err := root.GetForeignKeyCollection(ctx)
	require.NoError(t, err)

	assert.Equal(t, test.fks, fkc.AllKeys())

	for _, fk := range test.fks {
		// verify parent index
		pt, _, ok, err := root.GetTableInsensitive(ctx, fk.ReferencedTableName)
		require.NoError(t, err)
		require.True(t, ok)
		ps, err := pt.GetSchema(ctx)
		require.NoError(t, err)
		pi, ok := ps.Indexes().GetByNameCaseInsensitive(fk.ReferencedTableIndex)
		require.True(t, ok)
		require.Equal(t, fk.ReferencedTableColumns, pi.IndexedColumnTags())

		// verify child index
		ct, _, ok, err := root.GetTableInsensitive(ctx, fk.TableName)
		require.NoError(t, err)
		require.True(t, ok)
		cs, err := ct.GetSchema(ctx)
		require.NoError(t, err)
		ci, ok := cs.Indexes().GetByNameCaseInsensitive(fk.TableIndex)
		require.True(t, ok)
		require.Equal(t, fk.TableColumns, ci.IndexedColumnTags())
	}
}

var foreignKeyTests = []foreignKeyTest{
	{
		name: "create foreign key",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v1_idx (v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child add 
				constraint child_fk foreign key (v1) references parent(v1)`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "child_fk",
				TableName: "child",
				TableIndex: "v1_idx",
				TableColumns: []uint64{11},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
		},
	},
	{
		name: "create multi-column foreign key",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table parent add index v1v2_idx (v1, v2)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v1v2_idx (v1, v2)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child add 
				constraint multi_col foreign key (v1, v2) references parent(v1, v2)`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "multi_col",
				TableName: "child",
				TableIndex: "v1v2_idx",
				TableColumns: []uint64{11, 12},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1v2_idx",
				ReferencedTableColumns: []uint64{1, 2},
			},
		},
	},
	{
		name: "create multiple foreign keys",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v1_idx (v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v2_idx (v2)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add constraint fk1 foreign key (v1) references parent(v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add constraint fk2 foreign key (v2) references parent(v2)`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "fk1",
				TableName: "child",
				TableIndex: "v1_idx",
				TableColumns: []uint64{11},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
			{
				Name: "fk2",
				TableName: "child",
				TableIndex: "v2_idx",
				TableColumns: []uint64{12},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v2_idx",
				ReferencedTableColumns: []uint64{2},
			},
		},
	},
	{
		name: "create table with foreign key",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `create table new_table (
				id int comment 'tag:20',
				v1 int comment 'tag:21',
				constraint new_fk foreign key (v1) references parent(v1),
				primary key(id));`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "new_fk",
				TableName: "new_table",
				// unnamed indexes take the column name
				TableIndex: "v1",
				TableColumns: []uint64{21},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
		},
	},
	{
		name: "create foreign keys with update or delete rules",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v1_idx (v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v2_idx (v2)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add constraint fk1 foreign key (v1) references parent(v1) on update cascade`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add constraint fk2 foreign key (v2) references parent(v2) on delete set null`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "fk1",
				TableName: "child",
				TableIndex: "v1_idx",
				TableColumns: []uint64{11},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
				OnUpdate: doltdb.ForeignKeyReferenceOption_Cascade,
			},
			{
				Name: "fk2",
				TableName: "child",
				TableIndex: "v2_idx",
				TableColumns: []uint64{12},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v2_idx",
				ReferencedTableColumns: []uint64{2},
				OnDelete: doltdb.ForeignKeyReferenceOption_SetNull,
			},
		},
	},
	{
		name: "create single foreign key with update and delete rules",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v1_idx (v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add constraint child_fk foreign key (v1) references parent(v1) on update cascade on delete cascade`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "child_fk",
				TableName: "child",
				TableIndex: "v1_idx",
				TableColumns: []uint64{11},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
				OnUpdate: doltdb.ForeignKeyReferenceOption_Cascade,
				OnDelete: doltdb.ForeignKeyReferenceOption_Cascade,
			},
		},
	},
	{
		name: "create foreign keys with all update and delete rules",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", "alter table parent add column v3 int comment 'tag:3';"}},
			{commands.SqlCmd{}, []string{"-q", "alter table parent add column v4 int comment 'tag:4';"}},
			{commands.SqlCmd{}, []string{"-q", "alter table parent add column v5 int comment 'tag:5';"}},
			{commands.SqlCmd{}, []string{"-q", "alter table parent add index v3_idx (v3);"}},
			{commands.SqlCmd{}, []string{"-q", "alter table parent add index v4_idx (v4);"}},
			{commands.SqlCmd{}, []string{"-q", "alter table parent add index v5_idx (v5);"}},
			{commands.SqlCmd{}, []string{"-q", `create table sibling (
					id int comment 'tag:20',
					v1 int comment 'tag:21',
					v2 int comment 'tag:22',
					v3 int comment 'tag:23',
					v4 int comment 'tag:24',
					v5 int comment 'tag:25',
					constraint fk1 foreign key (v1) references parent(v1),
					constraint fk2 foreign key (v2) references parent(v2) on delete restrict on update restrict,
					constraint fk3 foreign key (v3) references parent(v3) on delete cascade on update cascade,
					constraint fk4 foreign key (v4) references parent(v4) on delete set null on update set null,
					constraint fk5 foreign key (v5) references parent(v5) on delete no action on update no action,
					primary key (id));`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "fk1",
				TableName: "sibling",
				TableIndex: "v1",
				TableColumns: []uint64{21},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
			{
				Name: "fk2",
				TableName: "sibling",
				TableIndex: "v2",
				TableColumns: []uint64{22},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v2_idx",
				ReferencedTableColumns: []uint64{2},
				OnUpdate: doltdb.ForeignKeyReferenceOption_Restrict,
				OnDelete: doltdb.ForeignKeyReferenceOption_Restrict,
			},
			{
				Name: "fk3",
				TableName: "sibling",
				TableIndex: "v3",
				TableColumns: []uint64{23},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v3_idx",
				ReferencedTableColumns: []uint64{3},
				OnUpdate: doltdb.ForeignKeyReferenceOption_Cascade,
				OnDelete: doltdb.ForeignKeyReferenceOption_Cascade,
			},
			{
				Name: "fk4",
				TableName: "sibling",
				TableIndex: "v4",
				TableColumns: []uint64{24},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v4_idx",
				ReferencedTableColumns: []uint64{4},
				OnUpdate: doltdb.ForeignKeyReferenceOption_SetNull,
				OnDelete: doltdb.ForeignKeyReferenceOption_SetNull,
			},
			{
				Name: "fk5",
				TableName: "sibling",
				TableIndex: "v5",
				TableColumns: []uint64{25},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v5_idx",
				ReferencedTableColumns: []uint64{5},
				OnUpdate: doltdb.ForeignKeyReferenceOption_NoAction,
				OnDelete: doltdb.ForeignKeyReferenceOption_NoAction,
			},
		},
	},
	{
		name: "create foreign key without preexisting child index",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table child add constraint child_fk foreign key (v1) references parent(v1)`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "child_fk",
				TableName: "child",
				// unnamed indexes take the column name
				TableIndex: "v1",
				TableColumns: []uint64{11},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
		},
	},
	{
		name: "create unnamed foreign key",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v1_idx (v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child add foreign key (v1) references parent(v1)`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "ajk4bsgi",
				TableName: "child",
				TableIndex: "v1_idx",
				TableColumns: []uint64{11},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
		},
	},
	{
		name: "create table with unnamed foreign key",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `create table new_table (
				id int comment 'tag:20',
				v1 int comment 'tag:21',
				foreign key (v1) references parent(v1),
				primary key(id));`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "7l96tsms",
				TableName: "new_table",
				// unnamed indexes take the column name
				TableIndex: "v1",
				TableColumns: []uint64{21},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
		},
	},
	{
		name: "create unnamed multi-column foreign key",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table parent add index v1v2_idx (v1, v2)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add index v1v2_idx (v1, v2)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add foreign key (v1, v2) references parent(v1, v2)`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "ltbb4q51",
				TableName: "child",
				TableIndex: "v1v2_idx",
				TableColumns: []uint64{11, 12},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1v2_idx",
				ReferencedTableColumns: []uint64{1, 2},
			},
		},
	},
	{
		name: "create multiple unnamed foreign keys",
		setup: []testCommand{
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v1_idx (v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child add index v2_idx (v2)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add foreign key (v1) references parent(v1)`}},
			{commands.SqlCmd{}, []string{"-q", `alter table child 
				add foreign key (v2) references parent(v2)`}},
		},
		fks: []doltdb.ForeignKey{
			{
				Name: "ajk4bsgi",
				TableName: "child",
				TableIndex: "v1_idx",
				TableColumns: []uint64{11},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v1_idx",
				ReferencedTableColumns: []uint64{1},
			},
			{
				Name: "jui84jda",
				TableName: "child",
				TableIndex: "v2_idx",
				TableColumns: []uint64{12},
				ReferencedTableName: "parent",
				ReferencedTableIndex: "v2_idx",
				ReferencedTableColumns: []uint64{2},
			},
		},
	},
}